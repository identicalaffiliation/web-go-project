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
- В конце команда печатает значения `AUTH_JWT_PRIVATE_KEY_PATH`, `AUTH_JWT_PUBLIC_KEY_PATH`,
  `AUTH_JWT_KEY_ID` и `AUTH_JWT_ISSUER` для auth.
- Validator нужен только `secrets/jwt/public.pem` (формат PKIX PEM). Приватный ключ ему не передавать.
