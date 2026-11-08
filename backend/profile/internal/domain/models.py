import uuid
from dataclasses import dataclass
from datetime import datetime
from enum import Enum
from typing import Literal
from uuid import UUID

from pydantic import BaseModel


class ProfileType(str, Enum):
    CUSTOMER = "customer"
    BUSINESS = "business"


class UserRole(str, Enum):
    """Роль пользователя из JWT (выдаёт auth, прокидывает шлюз в X-User-Role)."""

    CUSTOMER = "customer"
    BUSINESS = "business"


@dataclass
class Profile:
    id: uuid.UUID
    user_id: uuid.UUID
    profile_type: ProfileType

    first_name: str | None = None
    last_name: str | None = None
    phone: str | None = None
    company_name: str | None = None

    created_at: datetime | None = None
    updated_at: datetime | None = None


@dataclass
class OrderDTO:
    id: str
    product_id: str
    quantity: int
    total_price: int
    status: str
    created_at: datetime


@dataclass
class BalanceDTO:
    amount: int


@dataclass
class BusinessProductDTO:
    product_id: str
    title: str
    description: str | None
    price: int
    image_url: str | None
    rating: float
    feedbacks_count: int
    created_at: datetime
    updated_at: datetime


@dataclass
class BusinessProductsPageDTO:
    products: list[BusinessProductDTO]
    next_cursor: str | None


@dataclass
class WishlistItemDTO:
    product_id: str
    added_at: datetime


class LikeEvent(BaseModel):
    user_id: UUID
    product_id: UUID
    action: Literal["add", "remove"]
