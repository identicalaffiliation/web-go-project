import uuid

from fastapi import Header, HTTPException

# Предполагаем, что у нас есть слой сервиса (напишем на шаге 4)
# from internal.app.service import ProfileService


async def get_current_user_id(authorization: str = Header(...)) -> uuid.UUID:
    """
    Извлекает токен из заголовка и валидирует его.
    В реальном коде здесь будет jwt.decode(...) с ключом JWT_SECRET_KEY.
    """
    if not authorization.startswith("Bearer "):
        raise HTTPException(status_code=401, detail="Invalid token format")

    token = authorization.split(" ")[1]
    # TODO: Добавить реальную валидацию JWT
    # payload = jwt.decode(token, settings.JWT_SECRET_KEY, algorithms=["HS256"])
    # return uuid.UUID(payload["user_id"])

    # Пока возвращаем заглушку для локальных тестов
    return uuid.UUID("a1b2c3d4-e5f6-4a5b-8c9d-0123456789ab")


# Заглушка для получения бизнес-логики (реализуем сборку в main.py позже)
async def get_profile_service():
    # return ProfileService(...)
    pass
