// Нагрузка на orders: 500 rps создания заказа на 1 штуку товара.
// Каждый заказ запускает сагу (inventory -> payments).
//
// Сток товара в inventory не больше 99 (CHECK quantity < 100), поэтому товаров создаётся
// столько, чтобы стока хватило на весь прогон: иначе саги падают на InsufficientStock и тест
// мерит компенсацию, а не обычный путь. Заказы раскиданы по кругу по товарам и покупателям.
//
// Запуск: k6 run load/k6/orders.js
// Параметры (-e):
//   RATE=500 DURATION=1m - rps и длительность
//   PRODUCTS=N           - сколько товаров, по умолчанию считается из RATE и DURATION
//   CUSTOMERS=50         - сколько покупателей. Стартовый баланс 1 000 000, товар стоит 100:
//                          один покупатель выдерживает 10 000 заказов
//   ASYNC_WAIT=5         - сколько секунд ждать после setup: сток (catalog -> kafka -> inventory)
//                          и счета (auth -> kafka -> payments) создаются асинхронно
import http from 'k6/http';
import exec from 'k6/execution';
import { check, sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import {
  BASE_URL,
  DURATION,
  authHeaders,
  createProduct,
  registerAndLogin,
  registerBusiness,
  weightedOptions,
} from './lib/common.js';

const RATE = Number(__ENV.RATE || 500);
const CUSTOMERS = Number(__ENV.CUSTOMERS || 50);
const ASYNC_WAIT = Number(__ENV.ASYNC_WAIT || 5);
const MAX_STOCK = 99;

// '1m30s' -> 90
function durationSeconds(d) {
  const units = { h: 3600, m: 60, s: 1 };
  let total = 0;
  for (const [, n, u] of d.matchAll(/(\d+)([hms])/g)) {
    total += Number(n) * units[u];
  }

  return total;
}

// запас 20%: constant-arrival-rate может чуть перебрать итераций
const PRODUCTS = Number(__ENV.PRODUCTS || Math.ceil((RATE * durationSeconds(DURATION) * 1.2) / MAX_STOCK));

export const options = weightedOptions(RATE, { createOrder: 1 });

export function setup() {
  console.log(`setup: products=${PRODUCTS}, customers=${CUSTOMERS}`);

  const seller = registerBusiness();
  const productIds = [];
  for (let i = 0; i < PRODUCTS; i++) {
    productIds.push(createProduct(seller.token));
  }

  const tokens = [];
  for (let i = 0; i < CUSTOMERS; i++) {
    tokens.push(registerAndLogin().token);
  }

  sleep(ASYNC_WAIT);

  return { productIds, tokens };
}

// товары берутся по кругу общим номером итерации: сток расходуется равномерно, а не случайно
export function createOrder(data) {
  const i = exec.scenario.iterationInTest;

  const res = http.post(
    `${BASE_URL}/api/v1/orders`,
    JSON.stringify({
      productId: data.productIds[i % data.productIds.length],
      operationId: uuidv4(), // ключ идемпотентности, у каждого заказа свой
      quantity: 1,
    }),
    { headers: authHeaders(data.tokens[i % data.tokens.length]), tags: { name: 'POST /orders' } },
  );

  check(res, { 'order 201': (r) => r.status === 201 });
}
