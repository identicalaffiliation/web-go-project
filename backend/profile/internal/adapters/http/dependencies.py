import uuid

from fastapi import Header, HTTPException


def get_current_user_id(x_user_id: str = Header(..., alias="X-User-Id")) -> uuid.UUID:
    """
    Извлекает ID пользователя из заголовка X-User-Id,
    который прокидывает API-шлюз после успешной авторизации.
    """
    try:
        return uuid.UUID(x_user_id)
    except ValueError:
        raise HTTPException(
            status_code=400, detail="Invalid X-User-Id format"
        ) from None


# Заглушка для получения бизнес-логики (переопределяется в main.py)
async def get_profile_service():
    raise NotImplementedError("This dependency should be overridden in main.py")
