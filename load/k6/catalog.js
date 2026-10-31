// Нагрузка на catalog: все ручки, по умолчанию 2000 rps суммарно.
//
//   page           600 rps  GET  /api/v1/catalog?limit=20[&cursor=]   публичная
//   item           600 rps  GET  /api/v1/catalog/{id}
//   feedbacks      300 rps  GET  /api/v1/catalog/{id}/feedbacks
//   like           200 rps  POST /api/v1/catalog/like                 customer
//   createFeedback 150 rps  POST /api/v1/catalog/feedbacks            customer
//   create         100 rps  POST /api/v1/catalog                      business
//   image           50 rps  POST /api/v1/catalog/{id}/image           business
//
// Все ручки, кроме page, проходят через auth_request в gateway: auth получает ~1400 rps validate.
//
// Запуск: k6 run load/k6/catalog.js
// Параметры (-e):
//   RATE=2000 DURATION=1m - общий rps (делится по долям выше) и длительность
//   PRODUCTS=50           - сколько товаров создать заранее для чтения, лайков и отзывов
//   CUSTOMERS=20          - сколько покупателей (лайки, отзывы, карточки)
import http from 'k6/http';
import { check } from 'k6';
import {
  BASE_URL,
  authHeaders,
  createProduct,
  pick,
  registerAndLogin,
  registerBusiness,
  weightedOptions,
} from './lib/common.js';

const PRODUCTS = Number(__ENV.PRODUCTS || 50);
const CUSTOMERS = Number(__ENV.CUSTOMERS || 20);

export const options = weightedOptions(2000, {
  page: 0.3,
  item: 0.3,
  feedbacks: 0.15,
  like: 0.1,
  createFeedback: 0.075,
  create: 0.05,
  image: 0.025,
});

export function setup() {
  const seller = registerBusiness();

  const productIds = [];
  for (let i = 0; i < PRODUCTS; i++) {
    productIds.push(createProduct(seller.token));
  }

  const customers = [];
  for (let i = 0; i < CUSTOMERS; i++) {
    customers.push(registerAndLogin().token);
  }

  return { sellerToken: seller.token, productIds, customers };
}

// половина запросов - первая страница, половина - "глубже": курсор = id товара из пула
export function page(data) {
  const cursor = Math.random() < 0.5 ? '' : `&cursor=${pick(data.productIds)}`;
  const res = http.get(`${BASE_URL}/api/v1/catalog?limit=20${cursor}`, {
    tags: { name: 'GET /catalog' },
  });
  check(res, { 'page 200': (r) => r.status === 200 });
}

export function item(data) {
  const res = http.get(`${BASE_URL}/api/v1/catalog/${pick(data.productIds)}`, {
    headers: authHeaders(pick(data.customers)),
    tags: { name: 'GET /catalog/{id}' },
  });
  check(res, { 'item 200': (r) => r.status === 200 });
}

export function feedbacks(data) {
  const res = http.get(`${BASE_URL}/api/v1/catalog/${pick(data.productIds)}/feedbacks?limit=20`, {
    headers: authHeaders(pick(data.customers)),
    tags: { name: 'GET /catalog/{id}/feedbacks' },
  });
  check(res, { 'feedbacks 200': (r) => r.status === 200 });
}

export function like(data) {
  const res = http.post(
    `${BASE_URL}/api/v1/catalog/like`,
    JSON.stringify({ id: pick(data.productIds), action: Math.random() < 0.7 ? 'add' : 'remove' }),
    { headers: authHeaders(pick(data.customers)), tags: { name: 'POST /catalog/like' } },
  );
  check(res, { 'like 201': (r) => r.status === 201 });
}

export function createFeedback(data) {
  const res = http.post(
    `${BASE_URL}/api/v1/catalog/feedbacks`,
    JSON.stringify({
      productId: pick(data.productIds),
      score: 1 + Math.floor(Math.random() * 5),
      description: 'k6 feedback',
    }),
    { headers: authHeaders(pick(data.customers)), tags: { name: 'POST /catalog/feedbacks' } },
  );
  check(res, { 'feedback 201': (r) => r.status === 201 });
}

export function create(data) {
  createProduct(data.sellerToken, false);
}

export function image(data) {
  const res = http.post(
    `${BASE_URL}/api/v1/catalog/${pick(data.productIds)}/image?format=${Math.random() < 0.5 ? 'png' : 'jpeg'}`,
    null,
    { headers: authHeaders(data.sellerToken), tags: { name: 'POST /catalog/{id}/image' } },
  );
  check(res, { 'image 200': (r) => r.status === 200 });
}
