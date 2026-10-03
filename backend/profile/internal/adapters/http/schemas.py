import uuid
from datetime import datetime

from internal.domain.models import ProfileType
from pydantic import BaseModel, ConfigDict, Field


class ProfileResponse(BaseModel):
    """Схема для отправки данных профиля фронтенду."""

    id: uuid.UUID
    user_id: uuid.UUID
    profile_type: ProfileType
    first_name: str | None
    last_name: str | None
    phone: str | None
    company_name: str | None
    created_at: datetime
    updated_at: datetime

    model_config = ConfigDict(from_attributes=True)


class ProfileUpdateRequest(BaseModel):
    """Схема для обновления профиля. Все поля опциональны (PATCH-запрос)."""

    first_name: str | None = Field(default=None, min_length=1, max_length=100)
    last_name: str | None = Field(default=None, min_length=3, max_length=100)
    phone: str | None = Field(default=None, pattern=r"^(\+7|8|7)\d{10}$")
    company_name: str | None = None
