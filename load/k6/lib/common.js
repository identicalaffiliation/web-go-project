// Общие штуки для всех k6 скриптов.
import http from 'k6/http';
import { check, sleep } from 'k6';
import encoding from 'k6/encoding';

// Адрес gateway. Переопределяется так: k6 run -e BASE_URL=http://host script.js
export const BASE_URL = __ENV.BASE_URL || 'http://localhost';

// По умолчанию 500 rps на 5 минут.
export function loadOptions() {
  return {
    setupTimeout: '2m', // в setup ждём, пока auth выдаст роль business
    scenarios: {
      load: {
        executor: 'constant-arrival-rate', // держим постоянный rps, не зависящий от скорости ответа
        rate: Number(__ENV.RATE || 500), // сколько итераций в секунду
        timeUnit: '1s',
        duration: __ENV.DURATION || '5m',
        preAllocatedVUs: 100, // сколько виртуальных юзеров создать заранее
        maxVUs: 1000, // если сервис тормозит, k6 добавит юзеров до этого предела
      },
    },
    thresholds: {
      http_req_failed: ['rate<0.01'], // меньше 1% ошибок
      http_req_duration: ['p(95)<500'], // 95% запросов быстрее 500ms
    },
  };
}

export const JSON_HEADERS = { 'Content-Type': 'application/json' };

export function authHeaders(token) {
  return { ...JSON_HEADERS, Authorization: `Bearer ${token}` };
}

function login(email, password) {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email, password }),
    { headers: JSON_HEADERS },
  );
  check(res, { 'login 200': (r) => r.status === 200 });

  return res.json('access_token');
}

// Достаёт роль из access token (JWT: header.payload.signature, payload в base64url)
function tokenRole(token) {
  const payload = JSON.parse(encoding.b64decode(token.split('.')[1], 'rawurl', 's'));
  return payload.role;
}

// Регистрирует нового пользователя (всегда customer) и логинит его.
// Возвращает email, пароль и access token.
export function registerAndLogin() {
  const email = `k6-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;
  const password = 'k6-password-123';

  const reg = http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({ email, password, role: 'customer' }),
    { headers: JSON_HEADERS },
  );
  check(reg, { 'register 201': (r) => r.status === 201 });

  return { email, password, token: login(email, password) };
}

// Делает пользователя бизнесом: создаёт бизнес-профиль в profile.
// Роль в auth меняется асинхронно (profile -> kafka -> auth), поэтому
// перелогиниваемся, пока в токене не появится role=business.
export function becomeBusiness(user) {
  const res = http.post(
    `${BASE_URL}/api/v1/profiles/business`,
    JSON.stringify({ company_name: 'k6 company' }),
    { headers: authHeaders(user.token) },
  );
  check(res, { 'business profile 201': (r) => r.status === 201 });

  for (let i = 0; i < 30; i++) {
    const token = login(user.email, user.password);
    if (tokenRole(token) === 'business') {
      return { ...user, token };
    }
    sleep(1);
  }

  throw new Error('user did not get business role in 30s');
}

// Регистрирует пользователя и сразу делает его бизнесом
export function registerBusiness() {
  return becomeBusiness(registerAndLogin());
}

// Создаёт товар от имени business пользователя. Возвращает id товара.
export function createProduct(token) {
  const res = http.post(
    `${BASE_URL}/api/v1/catalog`,
    JSON.stringify({ title: 'k6 product', price: 100, quantity: 99 }),
    { headers: authHeaders(token) },
  );
  check(res, { 'create product 201': (r) => r.status === 201 });

  return res.json('product.productId');
}
