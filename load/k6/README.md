# k6 нагрузочные тесты

Простые скрипты на HTTP ручки сервисов. Все запросы идут через gateway (nginx).

| Скрипт       | Что нагружает                                              |
|--------------|------------------------------------------------------------|
| `auth.js`    | `POST /api/v1/auth/login`                                  |
| `catalog.js` | `GET /api/v1/catalog` и `GET /api/v1/catalog/{id}`         |
| `orders.js`  | `POST /api/v1/orders` (запускает сагу заказа)              |
| `profile.js` | `GET /api/v1/profiles/me`, `/balance`, `/orders`           |

По умолчанию 500 rps в течение 5 минут. Порог: меньше 1% ошибок и p95 < 500ms.

## Локальный запуск

Поднять проект (`make up migrations-up init-kafka-topics run`), потом:

```sh
k6 run load/k6/catalog.js
k6 run -e RATE=50 -e DURATION=30s load/k6/catalog.js   # короткий прогон
k6 run -e BASE_URL=http://my-host load/k6/auth.js        # другой адрес gateway
```

## В CI

GitHub → Actions → **Load tests (k6)** → Run workflow. Можно выбрать сервис, rps и длительность.
Сам по себе workflow не запускается, на push в main не влияет.
