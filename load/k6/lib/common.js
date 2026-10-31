// Общие штуки для всех k6 скриптов.
import http from 'k6/http';
import { check, sleep } from 'k6';
import encoding from 'k6/encoding';

// Адрес gateway. Переопределяется так: k6 run -e BASE_URL=http://host script.js
export const BASE_URL = __ENV.BASE_URL || 'http://localhost';

// rate у каждого скрипта свой (catalog 500, остальные 50), по умолчанию 1 минута.
// Перебить: k6 run -e RATE=100 -e DURATION=30s script.js
export function loadOptions(rate) {
  return {
    setupTimeout: '2m', // в setup ждём, пока auth выдаст роль business
    scenarios: {
      load: {
        executor: 'constant-arrival-rate', // держим постоянный rps, не зависящий от скорости ответа
        rate: Number(__ENV.RATE || rate), // сколько итераций в секунду
        timeUnit: '1s',
        duration: __ENV.DURATION || '1m',
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

export const DURATION = __ENV.DURATION || '1m';

// Опции для нескольких ручек под общий rps: total делится по долям weights, у каждой ручки
// свой constant-arrival-rate сценарий. Медленная ручка не съедает rps соседних, а в отчёте
// пороги и метрики видны по каждой ручке (тег scenario). Имя сценария = имя exec функции.
// Общий rps перебивается -e RATE=..., доли при этом сохраняются.
export function weightedOptions(total, weights) {
  const rps = Number(__ENV.RATE || total);
  const scenarios = {};
  const thresholds = { http_req_failed: ['rate<0.01'] };

  for (const [name, weight] of Object.entries(weights)) {
    const rate = Math.max(1, Math.round(rps * weight));
    scenarios[name] = {
      executor: 'constant-arrival-rate',
      exec: name,
      rate,
      timeUnit: '1s',
      duration: DURATION,
      preAllocatedVUs: Math.max(5, Math.ceil(rate / 10)), // хватает при ответе ~100ms
      // при ответе дольше 500ms k6 упрётся в предел и начнёт пропускать итерации
      // (dropped_iterations) - это и есть сигнал, что сервис не держит rps
      maxVUs: Math.max(50, Math.ceil(rate / 2)),
    };
    thresholds[`http_req_failed{scenario:${name}}`] = ['rate<0.01'];
    thresholds[`http_req_duration{scenario:${name}}`] = ['p(95)<500'];
  }

  return { setupTimeout: '5m', scenarios, thresholds };
}

export function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
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
    JSON.stringify({ email, password }),
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
