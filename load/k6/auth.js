// Нагрузка на auth, по умолчанию 300 rps суммарно:
//   register  60 rps  каждый раз новый пользователь (bcrypt - самая тяжёлая ручка)
//   login    120 rps  логин одного и того же пользователя
//   refresh  120 rps  ротация refresh токена
// Запуск: k6 run load/k6/auth.js
// Параметры (-e): RATE=300 DURATION=1m - общий rps (делится по долям выше) и длительность
import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, JSON_HEADERS, registerAndLogin, weightedOptions } from './lib/common.js';

export const options = weightedOptions(300, {
  register: 0.2,
  login: 0.4,
  refresh: 0.4,
});

// setup выполняется один раз до начала нагрузки
export function setup() {
  return registerAndLogin();
}

function doLogin(user) {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email: user.email, password: user.password }),
    { headers: JSON_HEADERS },
  );
  check(res, { 'login 200': (r) => r.status === 200 });

  return res;
}

export function login(user) {
  doLogin(user);
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

// Refresh токен одноразовый: после ротации старый уже не принимается.
// Поэтому у каждого VU своя цепочка токенов, первый берём логином.
let refreshToken = null;

export function refresh(user) {
  if (refreshToken === null) {
    const res = doLogin(user);
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
