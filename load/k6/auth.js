// Нагрузка на auth тремя параллельными сценариями:
//   register — 20 rps, каждый раз новый пользователь
//   login    — 50 rps, логин одного и того же пользователя
//   refresh  — 50 rps, ротация refresh токена
// Запуск: k6 run load/k6/auth.js
// Свой rps на сценарий: -e REGISTER_RATE=10 -e LOGIN_RATE=100 -e REFRESH_RATE=100
// (RATE, как в остальных скриптах, перебивает все три сразу)
import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, JSON_HEADERS, loadOptions, registerAndLogin } from './lib/common.js';

const base = loadOptions(0).scenarios.load;

function scenario(fn, rate) {
  return { ...base, exec: fn, rate: Number(__ENV.RATE || rate) };
}

export const options = {
  scenarios: {
    register: scenario('register', __ENV.REGISTER_RATE || 20),
    login: scenario('login', __ENV.LOGIN_RATE || 50),
    refresh: scenario('refresh', __ENV.REFRESH_RATE || 50),
  },
  thresholds: {
    ...loadOptions(0).thresholds,
    // те же пороги отдельно по сценариям, чтобы было видно, какой просел
    'http_req_duration{scenario:register}': ['p(95)<500'],
    'http_req_duration{scenario:login}': ['p(95)<500'],
    'http_req_duration{scenario:refresh}': ['p(95)<500'],
  },
};

// setup выполняется один раз до начала нагрузки
export function setup() {
  return registerAndLogin();
}

function login(user) {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email: user.email, password: user.password }),
    { headers: JSON_HEADERS },
  );
  check(res, { 'login 200': (r) => r.status === 200 });

  return res;
}

export function register() {
  const email = `k6-${Date.now()}-${Math.floor(Math.random() * 1e6)}@example.com`;
  const res = http.post(
    `${BASE_URL}/api/v1/auth/register`,
    JSON.stringify({ email, password: 'k6-password-123' }),
    { headers: JSON_HEADERS },
  );

  check(res, { 'register 201': (r) => r.status === 201 });
}

export { login };

// Refresh токен одноразовый: после ротации старый уже не принимается.
// Поэтому у каждого VU своя цепочка токенов, первый берём логином.
let refreshToken = null;

export function refresh(user) {
  if (refreshToken === null) {
    const res = login(user);
    if (res.status !== 200) {
      return;
    }
    refreshToken = res.json('refresh_token');
  }

  const res = http.post(
    `${BASE_URL}/api/v1/auth/refresh`,
    JSON.stringify({ refresh_token: refreshToken }),
    { headers: JSON_HEADERS },
  );
  const ok = check(res, { 'refresh 200': (r) => r.status === 200 });

  // при ошибке цепочка порвана, на следующей итерации начнём с логина
  refreshToken = ok ? res.json('refresh_token') : null;
}
