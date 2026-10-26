// Нагрузка на auth: логин одного и того же пользователя.
// Запуск: k6 run load/k6/auth.js
import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, JSON_HEADERS, loadOptions, registerAndLogin } from './lib/common.js';

export const options = loadOptions(50);

// setup выполняется один раз до начала нагрузки
export function setup() {
  return registerAndLogin();
}

// default выполняется 50 раз в секунду
export default function (user) {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ email: user.email, password: user.password }),
    { headers: JSON_HEADERS },
  );

  check(res, { 'login 200': (r) => r.status === 200 });
}
