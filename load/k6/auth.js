// Нагрузка на auth, по умолчанию 300 rps суммарно:
//   register  60 rps  каждый раз новый пользователь (bcrypt - самая тяжёлая ручка)
//   login    120 rps  логин одного и того же пользователя
//   refresh  120 rps  ротация refresh токена
import http from 'k6/http';
import {
  BASE_URL,
  JSON_HEADERS,
  RATE_LIMITED,
  checkStatus,
  registerAndLogin,
  weightedOptions,
} from './lib/common.js';

export const options = weightedOptions(200, {
  register: 0.2,
  login: 0.4,
  refresh: 0.4,
});

export function setup() {
  return registerAndLogin();
}

function doLogin(user) {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email: user.email, password: user.password }),
    { headers: JSON_HEADERS },
  );
  checkStatus(res, 'login', 200);

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

  checkStatus(res, 'register', 201);
}

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
  if (checkStatus(res, 'refresh', 200)) {
    refreshToken = res.json('refresh_token');
  } else if (res.status !== RATE_LIMITED) {
    refreshToken = null;
  }
}
