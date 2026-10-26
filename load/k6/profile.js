// Нагрузка на profile: чтение своих профилей, баланса и заказов.
import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, authHeaders, loadOptions, registerAndLogin } from './lib/common.js';

export const options = loadOptions(50);

const PATHS = ['/api/v1/profiles/me', '/api/v1/profiles/balance', '/api/v1/profiles/orders'];

export function setup() {
  return registerAndLogin();
}

export default function (user) {
  // каждая итерация бьёт в одну случайную ручку из списка
  const path = PATHS[Math.floor(Math.random() * PATHS.length)];
  const res = http.get(`${BASE_URL}${path}`, { headers: authHeaders(user.token) });

  check(res, { 'status 200': (r) => r.status === 200 });
}
