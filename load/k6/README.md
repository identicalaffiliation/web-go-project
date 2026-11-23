# k6 нагрузочные тесты

Простые скрипты на HTTP ручки сервисов. Все запросы идут через gateway (nginx).

| Скрипт       | rps по умолчанию | Что нагружает |
|--------------|------------------|---------------|
| `catalog.js` | 2000 | все ручки: страница (600), карточка (600), отзывы товара (300), лайк (200), создание отзыва (150), создание товара (100), картинка (50) |
| `orders.js`  | 500  | `POST /api/v1/orders`, каждый заказ запускает сагу. Товаров создаётся столько, чтобы стока (≤ 99 на товар) хватило на весь прогон |
| `auth.js`    | 300  | `register` (60), `login` (120), `refresh` (120) |
| `profile.js` | 50   | `GET /api/v1/profiles/me`, `/balance`, `/orders` |

По умолчанию 1 минута. `RATE` задаёт общий rps скрипта, доли ручек сохраняются; `DURATION` — длительность.
У каждой ручки свой сценарий, пороги считаются по каждой отдельно: меньше 1% ошибок и p95 < 500ms.
Если сервис не успевает, k6 пишет `dropped_iterations` — заданный rps не выдержан.
Остальные параметры (число товаров, покупателей) — в шапке каждого скрипта.

Сам k6 на 2000 rps ест заметно CPU: запускать лучше не на той же машине, где тормозит docker,
или смотреть, что k6 не упирается в CPU раньше сервисов.

## Rate limit

Gateway ограничивает запросы (`nginx/templates/rate-limits.conf.template`, значения — env `RL_*` у `gateway` в `compose.yml`):

- **на сервис** — 30 r/s на все ручки сервиса, чтение тоже: `RL_AUTH`, `RL_PROFILE`, `RL_ORDERS`.
  У каталога два потолка: чтение (GET) 1000 r/s — `RL_CATALOG_READ`, запись 30 r/s — `RL_CATALOG_WRITE`;
- **на клиента** на ручках записи:

| Ручка | лимит |
|-------|-------|
| `POST /api/v1/orders` | 5 r/s на токен (`RL_ORDERS_USER`) |
| `POST /api/v1/profiles/business` | 1 r/s на токен (`RL_PROFILE_USER`) |
| `POST /api/v1/auth/register` | 5 r/s на IP (`RL_REGISTER_IP`) |
| `POST /api/v1/catalog`, `/catalog/*` (товар, картинка, лайк, отзыв) | 30 r/s на токен (`RL_CATALOG_WRITE_USER`) |

Сверх лимита (и burst) gateway сразу отвечает `429` с `Retry-After`, до сервисов запрос не доходит.
Проверка токена (`auth_request`) лимит auth не расходует.

В скриптах `429` — штатный ответ, а не ошибка (`checkStatus` в `lib/common.js`):

- проверка ручки проходит на ожидаемом статусе **или** `429` (`login 200 or 429`);
- на `429` отдельно проверяется, что ответил лимитер gateway: `rate limit: Retry-After`, `rate limit: gateway body`;
- в `http_req_failed` `429` не попадает, долю ответов лимитера показывает метрика `rate_limited`.

Так прогон со штатными лимитами проверяет сам лимитер. Но `429` отдаёт nginx за пару миллисекунд,
и сервисы такие запросы не видят, поэтому для прогона **на ёмкость сервисов** лимиты
поднимают в `.env` и пересоздают gateway (`rate_limited` тогда около 0%):

```sh
for v in RL_AUTH RL_CATALOG_READ RL_CATALOG_WRITE RL_PROFILE RL_ORDERS \
         RL_ORDERS_USER RL_PROFILE_USER RL_REGISTER_IP RL_CATALOG_WRITE_USER; do
  echo "$v=100000r/s" >> .env
done
docker compose up -d --force-recreate gateway
```

`setup` проходит при любых лимитах: на 429 он ждёт `Retry-After` и повторяет.
Убрать — удалить строки `RL_*` из `.env` и снова пересоздать gateway: значения gateway читает только
при создании контейнера, без пересоздания лимиты так и останутся поднятыми
(проверить: `docker exec gateway env | grep RL_`).

## Локальный запуск

Поднять проект (`make up migrations-up init-kafka-topics run`), потом:

```sh
k6 run load/k6/catalog.js
k6 run -e RATE=200 -e DURATION=30s load/k6/catalog.js  # короткий прогон
k6 run -e BASE_URL=http://my-host load/k6/auth.js        # другой адрес gateway
```

## В CI

GitHub → Actions → **Load tests (k6)** → Run workflow. Можно выбрать сервис, rps и длительность.
Сам по себе workflow не запускается, на push в main не влияет.
