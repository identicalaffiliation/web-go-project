// Нагрузка на catalog: страница товаров (публичная), карточка товара (с токеном)
// и создание товара (business токен).
// Запуск: k6 run load/k6/catalog.js
import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL, authHeaders, createProduct, loadOptions, registerBusiness } from './lib/common.js';

export const options = loadOptions(500);

export function setup() {
  const user = registerBusiness();
  const productId = createProduct(user.token);

  return { token: user.token, productId };
}

export default function (data) {
  // 45% — страница, 45% — карточка, 10% — создание товара
  const r = Math.random();

  if (r < 0.1) {
    createProduct(data.token);
  } else if (r < 0.55) {
    const res = http.get(`${BASE_URL}/api/v1/catalog?limit=20`);
    check(res, { 'page 200': (r) => r.status === 200 });
  } else {
    const res = http.get(`${BASE_URL}/api/v1/catalog/${data.productId}`, {
      headers: authHeaders(data.token),
    });
    check(res, { 'item 200': (r) => r.status === 200 });
  }
}
