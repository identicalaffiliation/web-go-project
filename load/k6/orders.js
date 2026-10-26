// Нагрузка на orders: создание заказа на 1 штуку товара.
// Каждый заказ запускает сагу (inventory, payments).
// Запуск: k6 run load/k6/orders.js
import http from 'k6/http';
import { check } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import { BASE_URL, authHeaders, createProduct, loadOptions, registerAndLogin, registerBusiness } from './lib/common.js';

export const options = loadOptions();

export function setup() {
  const seller = registerBusiness();
  const productId = createProduct(seller.token);
  const customer = registerAndLogin();

  return { token: customer.token, productId };
}

export default function (data) {
  const res = http.post(
    `${BASE_URL}/api/v1/orders`,
    JSON.stringify({
      productId: data.productId,
      operationId: uuidv4(), // ключ идемпотентности, у каждого заказа свой
      quantity: 1,
    }),
    { headers: authHeaders(data.token) },
  );

  check(res, { 'order 201': (r) => r.status === 201 });
}
