import uuid
from abc import ABC, abstractmethod

from internal.domain.models import BalanceDTO, OrderDTO


class IOrdersClient(ABC):
    """Порт для получения данных из сервиса заказов."""

    @abstractmethod
    async def get_user_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        pass


class IPaymentsClient(ABC):
    """Порт для получения данных из сервиса платежей."""

    @abstractmethod
    async def get_user_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        pass
