import uuid

import jwt
from fastapi import Header, HTTPException
from internal.config.config import settings


def get_current_user_id(authorization: str = Header(...)) -> uuid.UUID:
    """
    Извлекает токен из заголовка Authorization, валидирует его криптографическую
    подпись и срок действия, после чего возвращает UUID пользователя.
    """
    if not authorization or not authorization.startswith("Bearer "):
        raise HTTPException(
            status_code=401,
            detail="Invalid or missing Authorization header. Expected format: 'Bearer <token>'",
        )

    token = authorization.split(" ")[1]

    try:
        payload = jwt.decode(token, settings.JWT_SECRET_KEY, algorithms=["RS256"])

        user_id_str = payload.get("user_id")

        if not user_id_str:
            raise HTTPException(
                status_code=401, detail="Token payload missing 'user_id'"
            )

        return uuid.UUID(user_id_str)

    except jwt.ExpiredSignatureError:
        raise HTTPException(status_code=401, detail="Token has expired")

    except jwt.InvalidTokenError:
        raise HTTPException(status_code=401, detail="Invalid token signature or format")

    except ValueError:
        # Сработает, если UUID(user_id_str) не сможет распарсить строку
        raise HTTPException(status_code=401, detail="Invalid user_id format in token")


# Заглушка для получения бизнес-логики (переопределяется в main.py)
async def get_profile_service():
    raise NotImplementedError("This dependency should be overridden in main.py")
