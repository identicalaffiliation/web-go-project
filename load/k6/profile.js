// Нагрузка на profile: чтение своих профилей, баланса и заказов.
import http from 'k6/http';
import { BASE_URL, authHeaders, checkStatus, loadOptions, registerAndLogin } from './lib/common.js';

export const options = loadOptions(300);

const PATHS = ['/api/v1/profiles/me', '/api/v1/profiles/balance', '/api/v1/profiles/orders'];

export function setup() {
  return registerAndLogin();
}

export default function (user) {
  const path = PATHS[Math.floor(Math.random() * PATHS.length)];
  const res = http.get(`${BASE_URL}${path}`, { headers: authHeaders(user.token) });

  checkStatus(res, 'status', 200);
}
