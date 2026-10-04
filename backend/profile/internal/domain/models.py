import uuid
from dataclasses import dataclass
from datetime import datetime
from enum import Enum


class ProfileType(str, Enum):
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
    price: float
    status: str
    created_at: datetime

@dataclass
class BalanceDTO:
    amount: float

@dataclass
class WishlistItemDTO:
    product_id: str
    added_at: datetime