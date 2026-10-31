# web-go-project

Бэкенд интернет-магазина на микросервисах (MVP). Пользователь регистрируется, получает стартовый баланс,
продавец заводит бизнес-профиль и выставляет товары, покупатель оформляет заказ — резерв стока и
списание денег выполняются распределённой сагой с компенсациями.

**Стек:** Go, Python (FastAPI), PostgreSQL (master/replica), Apache Kafka, Redis, MinIO (S3),
Temporal, gRPC, Nginx, Docker Compose, k6.

## Содержание

- [Архитектура](#архитектура)
- [Сервисы](#сервисы)
- [Сценарии](#сценарии)
- [Быстрый старт](#быстрый-старт)
- [API](#api)
- [Конфигурация](#конфигурация)
- [Порты и UI](#порты-и-ui)
- [Разработка](#разработка)
- [Структура репозитория](#структура-репозитория)
- [Ограничения MVP](#ограничения-mvp)

## Архитектура

```
                         ┌──────────────────────────────┐
  client ──HTTP──▶ gateway (nginx :80) ── auth_request ──▶ auth /validate
                         │  X-User-Id, X-User-Role
        ┌────────────────┼──────────────┬──────────────┐
        ▼                ▼              ▼              ▼
      auth           catalog         profile         orders
   (Go, REST)     (Go, REST+gRPC)  (Python, REST)  (Go, REST+gRPC)
        │                │              │   gRPC       │  gRPC: цена товара
        │                │              ├────────────▶ orders.GetUserOrders
        │                │              └────────────▶ payments.GetBalanceByUser
        │                │                             │
        │                │                             ▼ Temporal: OrderSaga
        │                │                ┌────────────┴────────────┐
        │                │                ▼                         ▼
        │                │            inventory                  payments
        │                │         (Go, Temporal worker)   (Go, gRPC + Temporal worker)
        │                │                ▲                         ▲
        └──── Kafka ─────┴──── Kafka ─────┘           Kafka ────────┘
```

Принципы:

- **Единая точка входа** — nginx. Он проверяет JWT через `auth_request` к auth и прокидывает
  в сервисы заголовки `X-User-Id` / `X-User-Role`. Значения этих заголовков от клиента всегда
  перезаписываются, подделать их снаружи нельзя.
- **Database per service** — у каждого сервиса своя PostgreSQL. У catalog, orders и payments есть
  реплика для чтения.
- **Transactional outbox** — auth и catalog пишут события в outbox-таблицу в той же транзакции,
  что и данные; фоновый воркер публикует их в Kafka.
- **Сага на Temporal** — оформление заказа оркестрирует orders; inventory и payments выполняют
  свои шаги как Temporal-активити на своих task queue.
- **Гексагональная архитектура** в Go-сервисах: `domain` → `ports` → `service`/`app` → `adapters`.

## Сервисы

| Сервис      | Язык   | Интерфейсы                       | БД                  | Что делает |
|-------------|--------|----------------------------------|---------------------|------------|
| `auth`      | Go     | REST `:8080`                     | `auth-db`           | Регистрация, логин, JWT (RS256), refresh-токены с ротацией, валидация токена для gateway. Публикует `users.created`, слушает `profiles.business.created` и выдаёт роль `business`. |
| `profile`   | Python | REST `:8000`                     | `profile-db`        | Профили пользователя (customer / business). Создаёт профиль по `users.created`, публикует `profiles.business.created`. Агрегирует заказы и баланс через gRPC. |
| `catalog`   | Go     | REST `:8080`, gRPC `:50051`      | `catalog-db` + реплика, Redis, MinIO | Товары: создание, страница с курсором, карточка, загрузка картинки (pre-signed URL в MinIO). Кэш в Redis. Отдаёт цену по gRPC. Публикует `stock.created`. |
| `inventory` | Go     | Kafka consumer, Temporal worker  | `inventory-db`      | Остатки товаров. Заводит сток по `stock.created`; активити `ReserveStock` / `ReleaseStock`. |
| `orders`    | Go     | REST `:8080`, gRPC `:50052`, Temporal worker | `orders-db` + реплика | Создание заказа (с ключом идемпотентности), запуск и ведение саги, история заказов по gRPC. |
| `payments`  | Go     | gRPC `:50052`, Kafka consumer, Temporal worker | `payments-db` + реплика | Счета пользователей. Заводит счёт со стартовым балансом по `users.created`; активити `Charge` / `Refund`; баланс по gRPC. |
| `migrator`  | Go     | —                                | все                 | Накатывает SQL-миграции из `migrator/migrations/<service>`. |
| `gateway`   | nginx  | HTTP `:80`                       | —                   | Маршрутизация, авторизация через `auth_request`. |

### Kafka-топики

| Топик                       | Producer | Consumers          | Событие |
|-----------------------------|----------|--------------------|---------|
| `users.created`             | auth     | profile, payments  | Зарегистрирован пользователь |
| `profiles.business.created` | profile  | auth               | Создан бизнес-профиль → выдать роль `business` |
| `stock.created`             | catalog  | inventory          | Создан товар → завести остаток |

### gRPC-контракты

Лежат в `backend/proto/v1`:

| Сервис                                 | Метод                  | Кто вызывает |
|----------------------------------------|------------------------|--------------|
| `catalog.price.v1.PriceService`        | `GetPriceByProductId`  | orders |
| `orders.orders.v1.OrdersService`       | `GetUserOrders`        | profile |
| `payments.balance.v1.PaymentsService`  | `GetBalanceByUser`     | profile |

## Сценарии

### Регистрация

1. `POST /api/v1/auth/register` — auth создаёт пользователя с ролью `customer` и событие в outbox.
2. Outbox-воркер публикует `users.created`.
3. profile создаёт customer-профиль, payments — счёт со стартовым балансом (`initial_balance`, по умолчанию `1000000`).

### Стать продавцом

1. `POST /api/v1/profiles/business` — profile создаёт бизнес-профиль и публикует `profiles.business.created`.
2. auth выдаёт пользователю роль `business`.
3. Роль попадает в токен при следующем `login` / `refresh` — после этого доступны ручки продавца.

### Выставить товар

1. `POST /api/v1/catalog` (роль `business`) — catalog сохраняет товар и событие в outbox.
2. Outbox-воркер публикует `stock.created`, inventory заводит остаток.
3. `POST /api/v1/catalog/{productId}/image?format=png` — получить pre-signed URL и загрузить картинку в MinIO.

### Оформить заказ (сага)

1. `POST /api/v1/orders` (роль `customer`) — orders получает цену из catalog по gRPC, сохраняет заказ
   со статусом `created` и асинхронно запускает workflow `orders.OrderSaga`. `operationId` — ключ
   идемпотентности: повтор с тем же ключом даёт `409`.
2. Сага: `inventory.ReserveStock` → `payments.Charge` → статус `paid`.
3. При ошибке любого шага — компенсация: `payments.Refund` → `inventory.ReleaseStock` → статус `failed`.
   Компенсирующие шаги идемпотентны и выполняются на отвязанном контексте, поэтому отрабатывают
   даже при отмене workflow. Бизнес-ошибки (нет стока, не хватает денег) не ретраятся.
4. Итоговый статус и баланс видны в `GET /api/v1/profiles/orders` и `GET /api/v1/profiles/balance`.

## Быстрый старт

### Требования

- Docker и Docker Compose
- Go 1.26+ (для `make keys`, тестов и локального запуска)
- make
- опционально: golangci-lint v2, Python 3.14 + ruff (линт profile), protoc + mockery + swag (кодогенерация), k6

### Запуск

```sh
cp .env.example .env     # при необходимости поправить пароли и DSN
make keys                # RSA-ключи для JWT в secrets/jwt
make up                  # инфраструктура: postgres, kafka, redis, minio, temporal
make migrations-up       # миграции всех БД
make init-kafka-topics   # топики kafka
make run                 # сервисы + gateway
```

Проверка:

```sh
curl http://localhost/healthz   # ok
```

Остановить всё и удалить данные: `make clean`. Список команд: `make help`.

> В `.env` должны быть заданы DSN для orders — `MASTER_ORDERS_POSTGRES_DSN` и
> `REPLICA_ORDERS_POSTGRES_DSN` (их читает `compose.yml`):
>
> ```
> MASTER_ORDERS_POSTGRES_DSN=postgres://postgres:postgres@orders-master-postgres:5432/orders-db?sslmode=disable
> REPLICA_ORDERS_POSTGRES_DSN=postgres://postgres:postgres@orders-replica-postgres:5432/orders-db?sslmode=disable
> ```

### Пример сквозного сценария

```sh
# регистрация и логин
curl -X POST http://localhost/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"password123"}'

TOKEN=$(curl -s -X POST http://localhost/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"buyer@example.com","password":"password123"}' | jq -r .access_token)

# каталог (публичный)
curl 'http://localhost/api/v1/catalog?limit=10'

# заказ
curl -X POST http://localhost/api/v1/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"productId":"<uuid товара>","operationId":"<новый uuid>","quantity":1}'

# статус заказа и баланс
curl -H "Authorization: Bearer $TOKEN" http://localhost/api/v1/profiles/orders
curl -H "Authorization: Bearer $TOKEN" http://localhost/api/v1/profiles/balance
```

Точные поля запросов и ответов — в Swagger (см. ниже).

## API

Все запросы идут через gateway `http://localhost`. Защищённые ручки требуют
`Authorization: Bearer <access_token>`; без токена — `401`, с неподходящей ролью — `403`.

### Auth — `/api/v1/auth`

| Метод | Путь        | Доступ    | Описание |
|-------|-------------|-----------|----------|
| POST  | `/register` | публичный | Регистрация (роль `customer`). Пароль 8–72 байта. `201`, `409` если email занят |
| POST  | `/login`    | публичный | Пара токенов: access JWT + refresh |
| POST  | `/refresh`  | публичный | Новая пара по refresh-токену, старый отзывается (ротация) |
| POST  | `/logout`   | публичный | Отзыв refresh-токена, идемпотентно, `204` |

`/api/v1/auth/validate` — внутренняя ручка для nginx, снаружи отдаёт `404`.

### Catalog — `/api/v1/catalog`

| Метод | Путь                        | Доступ               | Описание |
|-------|-----------------------------|----------------------|----------|
| GET   | `/`                         | публичный            | Страница товаров: `limit` (по умолчанию 50), `cursor` из предыдущего ответа |
| GET   | `/{productId}`              | customer, business   | Карточка товара |
| POST  | `/`                         | business             | Создать товар: `title`, `description?`, `price > 0`, `quantity` 1–99 |
| POST  | `/{productId}/image`        | business             | Pre-signed URL для загрузки картинки, `format=jpeg\|png` |

### Profile — `/api/v1/profiles`

| Метод | Путь                | Доступ        | Описание |
|-------|---------------------|---------------|----------|
| GET   | `/me`               | авторизованный | Все профили пользователя |
| PATCH | `/me/{profile_id}`  | авторизованный | Обновить `first_name`, `last_name`, `phone` (`+7XXXXXXXXXX`), `company_name` |
| POST  | `/business`         | авторизованный | Создать бизнес-профиль, `409` если уже есть |
| GET   | `/orders`           | авторизованный | Заказы пользователя (через gRPC orders), `502` если orders недоступен |
| GET   | `/balance`          | авторизованный | Баланс (через gRPC payments), `502` если payments недоступен |

### Orders — `/api/v1/orders`

| Метод | Путь | Доступ   | Описание |
|-------|------|----------|----------|
| POST  | `/`  | customer | Создать заказ: `productId`, `operationId` (ключ идемпотентности), `quantity` 1–99. `201`, `409` при повторе `operationId` |

Статусы заказа: `created` → `paid` | `failed`.

### Swagger

| Сервис  | URL |
|---------|-----|
| auth    | файлы `backend/auth/docs/auth/swagger.{json,yaml}` |
| catalog | http://localhost/api/v1/catalog/swagger/index.html |
| orders  | http://localhost/api/v1/orders/swagger/index.html |
| profile | `backend/profile/openapi.json` |

Перегенерировать: `make swag`.

## Конфигурация

Go-сервисы читают несекретные настройки из `backend/<service>/configs/<service>.yml`
(в compose монтируется в `/app/config.yml`), секреты и DSN — из переменных окружения.
Profile настраивается только переменными окружения (`POSTGRES_*`, `KAFKA_*`, `*_GRPC_HOST`).

Основные переменные (`.env`, см. `.env.example`):

| Переменная | Кому | Назначение |
|------------|------|------------|
| `MASTER_AUTH_POSTGRES_DSN` | auth | DSN базы auth |
| `MASTER_CATALOG_POSTGRES_DSN`, `REPLICA_CATALOG_POSTGRES_DSN` | catalog | master / replica |
| `MASTER_ORDERS_POSTGRES_DSN`, `REPLICA_ORDERS_POSTGRES_DSN` | orders | master / replica |
| `MASTER_PAYMENTS_POSTGRES_DSN`, `REPLICA_PAYMENTS_POSTGRES_DSN` | payments | master / replica |
| `MINIO_USER`, `MINIO_PASSWORD` | minio, catalog | доступ к S3 |
| `REDIS_PASSWORD` | redis, catalog | пароль Redis |
| `AUTH_JWT_KEY_ID` | auth | `kid` в заголовке JWT, печатает `make keys` |

Полезные значения в конфигах:

- `auth.yml` — `access_ttl: 15m`, `refresh_ttl: 168h`, настройки outbox и consumer'а ролей.
  Любую настройку можно перебить переменной `AUTH_*` (имя указано в комментарии рядом).
- `payments.yml` — `initial_balance` (стартовый баланс нового пользователя).
- `catalog.yml` — TTL кэша, bucket MinIO, срок жизни pre-signed URL, параметры outbox.
- `orders.yml` / `inventory.yml` / `payments.yml` — адрес Temporal и task queue.

### JWT-ключи

Auth подписывает токены приватным RSA-ключом и проверяет их публичным.
Ключи у каждого свои и в git не попадают (`secrets/` в `.gitignore`).

```sh
make keys          # создаёт secrets/jwt/private.pem и secrets/jwt/public.pem
make keys-force    # перезаписывает существующие ключи (выданные токены перестанут работать)
```

То же без make: `go -C backend/auth run ./cmd/keygen -out ../../secrets/jwt`.

- Без `-force` существующие ключи не перезаписываются: команда завершится с ошибкой.
- В конце команда печатает `PRIVATE_KEY_PATH`, `PUBLIC_KEY_PATH`, `AUTH_JWT_KEY_ID` и `AUTH_JWT_ISSUER`.
- Тем, кто только проверяет токены, нужен только `public.pem` (PKIX PEM). Приватный ключ не передавать.

### Локальный запуск auth без Docker

Только из окружения читаются `MASTER_DSN`, `PRIVATE_KEY_PATH`, `PUBLIC_KEY_PATH` и `AUTH_JWT_KEY_ID` —
без них сервис не стартует. Сам `.env` сервис не читает.

```sh
docker compose up -d auth-postgres
docker compose up --build auth-migrator
make keys

cd backend/auth
export MASTER_DSN="postgres://postgres:postgres@localhost:5430/auth-db?sslmode=disable"
export PRIVATE_KEY_PATH="../../secrets/jwt/private.pem"
export PUBLIC_KEY_PATH="../../secrets/jwt/public.pem"
export AUTH_JWT_KEY_ID="<значение из make keys>"
go run ./cmd/auth -config configs/auth.yml
```

PowerShell: то же через `$env:MASTER_DSN = "..."` и т. д.

- Переменная с пустым значением (например, `AUTH_HTTP_ADDR=` в `.env`) считается заданной и даёт
  ошибку при старте — она не означает «взять значение из файла».
- Если файла по пути `-config` нет или значение не проходит проверку, сервис завершается с ошибкой,
  в которой названы путь или настройка.

## Порты и UI

| Что               | Адрес |
|-------------------|-------|
| Gateway           | http://localhost |
| Kafka UI          | http://localhost:4200 |
| Temporal UI       | http://localhost:8233 |
| MinIO Console     | http://localhost:9001 (`MINIO_USER` / `MINIO_PASSWORD`) |
| MinIO S3 API      | localhost:9000 |
| Redis             | localhost:6379 |
| Kafka             | localhost:9092 |
| Temporal (gRPC)   | localhost:7233 |
| orders REST       | через gateway: http://localhost/api/v1/orders (3 реплики, наружу порт не публикуется) |
| payments gRPC     | localhost:50053 |

PostgreSQL (пользователь `postgres` / `postgres`):

| БД                 | Порт |
|--------------------|------|
| auth               | 5430 |
| profile            | 5431 |
| catalog master / replica | 5432 / 5433 |
| inventory          | 5434 |
| temporal           | 5435 (`temporal` / `temporal`) |
| orders master / replica  | 5436 / 5437 |
| payments master / replica | 5438 / 5439 |

## Разработка

### Тесты

```sh
make unit           # unit-тесты Go-сервисов (-tags=unit, -race)
make integrations   # интеграционные тесты Go-сервисов (-tags=integration, нужен Docker — testcontainers)
make test           # всё вышеперечисленное + pytest для profile
```

### Линтеры

```sh
make lint           # golangci-lint для Go-сервисов, ruff для profile
```

Для одного сервиса: `cd backend/<service> && golangci-lint run -c configs/golangci.yml ./...`.

### Кодогенерация

```sh
make proto          # Go-код из backend/proto
make gen            # proto + моки (mockery, go generate)
make swag           # swagger
```

Клиент catalog для orders генерируется отдельно: `go_package` в proto указывает на модуль catalog.

### Миграции

SQL-файлы в `migrator/migrations/<service>/`. Новая миграция — новый файл с префиксом-таймстемпом
`YYYYMMDDHHMMSS_<name>.sql`. Накатить: `make migrations-up`.

### Нагрузочное тестирование

```sh
make load                          # k6 по auth, catalog, orders, profile по очереди
make load RATE=100 DURATION=30s    # свои rps и длительность
```

Пороги: меньше 1% ошибок и p95 < 500 ms. Подробности — `load/k6/README.md`.

### CI

- `.github/workflows/ci.yml` — линт, unit- и интеграционные тесты, `go vet` и сборка каждого
  Go-сервиса; линт profile.
- `.github/workflows/load.yml` — k6, запускается вручную (Actions → **Load tests (k6)** → Run workflow).
- `.github/workflows/prs_and_commits.yml` — проверки PR и коммитов.

## Структура репозитория

```
backend/
  auth/        Go: регистрация, JWT, refresh-токены
  catalog/     Go: товары, Redis, MinIO, gRPC цены
  inventory/   Go: остатки, Temporal-активити
  orders/      Go: заказы, сага на Temporal
  payments/    Go: счета, Temporal-активити, gRPC баланса
  profile/     Python (FastAPI): профили, агрегация заказов и баланса
  proto/       gRPC-контракты
migrator/      миграции всех БД
nginx/         конфиг gateway
scripts/       образы PostgreSQL с репликацией
load/k6/       нагрузочные сценарии
secrets/       JWT-ключи (не в git)
compose.yml    всё окружение
Makefile       команды проекта
```

Типовая структура Go-сервиса:

```
cmd/<service>/      точка входа
configs/            yml-конфиг, golangci, mockery
internal/
  domain/           сущности и бизнес-правила
  ports/            интерфейсы (inbound / outbound)
  service/ | app/   use case'ы
  adapters/         postgres, kafka, rest, rpc, temporal, s3, cache
  config/           загрузка конфига
  box/              сборка зависимостей
gen/                сгенерированный код (pb, моки)
tests/integration/  интеграционные тесты
```

## Ограничения MVP

- Заказ — один товар в одной позиции (`productId` + `quantity`).
- Оплата внутренняя: баланс на счёте payments, без внешнего платёжного провайдера.
- Роль `business` попадает в токен только после повторного `login` / `refresh`.
- Kafka, Redis и MinIO — по одному инстансу, replication factor топиков 1.
- Пароли и учётные данные в `compose.yml` — для локальной разработки, не для продакшена.
