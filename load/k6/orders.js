import http from 'k6/http';
import exec from 'k6/execution';
import { sleep } from 'k6';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';
import {
  BASE_URL,
  DURATION,
  authHeaders,
  checkStatus,
  createProduct,
  registerAndLogin,
  registerBusiness,
  weightedOptions,
} from './lib/common.js';

const RATE = Number(__ENV.RATE || 300);
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

  checkStatus(res, 'order', 201);
}
