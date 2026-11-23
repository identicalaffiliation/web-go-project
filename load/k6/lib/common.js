// Общие штуки для всех k6 скриптов.
import http from 'k6/http';
import { check, sleep } from 'k6';
import encoding from 'k6/encoding';
import { Rate } from 'k6/metrics';

// Адрес gateway. Переопределяется так: k6 run -e BASE_URL=http://host script.js
export const BASE_URL = __ENV.BASE_URL || 'http://localhost';

// 429 от rate limit gateway - штатный ответ: лимитер сработал, до сервиса запрос не дошёл.
export const RATE_LIMITED = 429;
http.setResponseCallback(http.expectedStatuses({ min: 200, max: 399 }, RATE_LIMITED));

const rateLimited = new Rate('rate_limited');

export function checkStatus(res, name, status) {
  const limited = res.status === RATE_LIMITED;
  rateLimited.add(limited);

  check(res, { [`${name} ${status} or 429`]: (r) => r.status === status || limited });
  if (limited) {
    check(res, {
      'rate limit: Retry-After': (r) => !!r.headers['Retry-After'],
      'rate limit: gateway body': (r) => r.json('message') === 'too many requests',
    });
  }

  return res.status === status;
}

export function loadOptions(rate) {
  return {
    setupTimeout: '2m',
    scenarios: {
      load: {
        executor: 'constant-arrival-rate',
        rate: Number(__ENV.RATE || rate),
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

function postRetry(url, body, params) {
  for (let i = 0; i < 60; i++) {
    const res = http.post(url, body, params);
    if (res.status !== 429) {
      return res;
    }
    sleep(Number(res.headers['Retry-After'] || 1));
  }

  throw new Error(`${url}: still 429 after 60 retries`);
}

export function authHeaders(token) {
  return { ...JSON_HEADERS, Authorization: `Bearer ${token}` };
}

function login(email, password) {
  const res = postRetry(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email, password }),
    { headers: JSON_HEADERS },
  );
  checkStatus(res, 'login', 200);

  return res.json('access_token');
}

function tokenRole(token) {
  const payload = JSON.parse(encoding.b64decode(token.split('.')[1], 'rawurl', 's'));
  return payload.role;
}

// Регистрирует нового пользователя (всегда customer) и логинит его.
// Возвращает email, пароль и access token.
export function registerAndLogin() {
  const email = `k6-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;
  const password = 'k6-password-123';

  const reg = postRetry(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({ email, password }),
    { headers: JSON_HEADERS },
  );
  checkStatus(reg, 'register', 201);

  return { email, password, token: login(email, password) };
}

// Делает пользователя бизнесом: создаёт бизнес-профиль в profile.
// Роль в auth меняется асинхронно (profile -> kafka -> auth), поэтому
// перелогиниваемся, пока в токене не появится role=business.
export function becomeBusiness(user) {
  const res = postRetry(
    `${BASE_URL}/api/v1/profiles/business`,
    JSON.stringify({ company_name: 'k6 company' }),
    { headers: authHeaders(user.token) },
  );
  checkStatus(res, 'business profile', 201);

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
export function createProduct(token, retry = true) {
  const post = retry ? postRetry : (url, body, params) => http.post(url, body, params);
  const res = post(
    `${BASE_URL}/api/v1/catalog`,
    JSON.stringify({ title: 'k6 product', price: 100, quantity: 99 }),
    { headers: authHeaders(token) },
  );
  checkStatus(res, 'create product', 201);

  return res.json('product.productId');
}
