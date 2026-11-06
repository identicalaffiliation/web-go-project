import uuid
from collections.abc import Callable

from fastapi import Header, HTTPException, status

from internal.domain.models import UserRole


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


def require_roles(*roles: UserRole) -> Callable[[str | None], None]:
    """
    Пропускает запрос, только если роль из заголовка X-User-Role,
    который прокидывает API-шлюз из JWT, входит в roles.
    Нет роли - 401, чужая роль - 403.
    """
    allowed = {r.value for r in roles}

    def check_role(
        x_user_role: str | None = Header(default=None, alias="X-User-Role"),
    ) -> None:
        if not x_user_role:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED, detail="Unauthorized"
            )
        if x_user_role not in allowed:
            raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN, detail="Forbidden"
            )

    return check_role


# Заглушка для получения бизнес-логики (переопределяется в main.py)
async def get_profile_service():
    raise NotImplementedError("This dependency should be overridden in main.py")
