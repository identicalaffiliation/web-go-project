# web-go-project

## JWT-ключи

Auth подписывает токены приватным RSA-ключом, validator проверяет их публичным.
Ключи у каждого свои, в git они не попадают (папка `secrets/` в `.gitignore`).

```
make keys          # создаёт secrets/jwt/private.pem и secrets/jwt/public.pem
make keys-force    # перезаписывает существующие ключи (выданные токены перестанут работать)
```

То же без make:

```
go -C backend/auth run ./cmd/keygen -out ../../secrets/jwt
```

- Без `-force` существующие ключи не перезаписываются: команда завершится с ошибкой.
- В конце команда печатает значения `PRIVATE_KEY_PATH`, `PUBLIC_KEY_PATH`,
  `AUTH_JWT_KEY_ID` и `AUTH_JWT_ISSUER` для auth.
- Validator нужен только `secrets/jwt/public.pem` (формат PKIX PEM). Приватный ключ ему не передавать.

## Локальный запуск auth service

Настройки берутся из двух мест:

- `backend/auth/configs/auth.yml` — все несекретные значения (адрес, таймауты, пул БД, TTL токенов,
  уровень логов). Путь к файлу задаёт флаг `-config`, по умолчанию `configs/auth.yml`.
- Переменные окружения `AUTH_*` — приоритетнее файла. Только из окружения читаются
  `MASTER_DSN`, `PRIVATE_KEY_PATH`, `PUBLIC_KEY_PATH` и `AUTH_JWT_KEY_ID`:
  без них сервис не стартует. Примеры есть в `.env.example`.

Сервис сам `.env` не читает: переменные нужно задать в оболочке, из которой он запускается.

1. Поднять базу и применить миграции (из корня репозитория):

   ```
   docker compose up -d auth-postgres
   docker compose up --build auth-migrator
   ```

2. Создать ключи, если их ещё нет: `make keys`. Команда напечатает `AUTH_JWT_KEY_ID`.

3. Задать переменные и запустить сервис из `backend/auth` (PowerShell):

   ```powershell
   cd backend/auth
   $env:MASTER_DSN = "postgres://postgres:<пароль>@localhost:5430/auth-db?sslmode=disable"
   $env:PRIVATE_KEY_PATH = "../../secrets/jwt/private.pem"
   $env:PUBLIC_KEY_PATH = "../../secrets/jwt/public.pem"
   $env:AUTH_JWT_KEY_ID = "<значение из make keys>"
   go run ./cmd/auth -config configs/auth.yml
   ```

   Переменные `$env:` живут только в текущем окне PowerShell.

- Переопределить настройку из файла: `$env:AUTH_HTTP_ADDR = ":9090"`. Имена переменных указаны
  в комментариях `configs/auth.yml`.
- Переменная с пустым значением (например, строка `AUTH_HTTP_ADDR=` в `.env` для compose) считается
  заданной и даёт ошибку при старте: она не означает «взять значение из файла».
- Если файла по пути из `-config` нет или значение не проходит проверку, сервис завершается
  с ошибкой, в которой назван путь или настройка.

Линтер (нужен golangci-lint v2), из `backend/auth`:

```
golangci-lint run -c configs/golangci.yml ./...
```
