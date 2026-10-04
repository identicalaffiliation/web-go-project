import uuid
from datetime import datetime, timezone

from internal.domain.models import BalanceDTO, OrderDTO
from internal.ports.rpc_clients import IOrdersClient, IPaymentsClient


class MockOrdersClient(IOrdersClient):
    async def get_user_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        # Возвращаем захардкоженный список заказов для фронтенда
        return [
            OrderDTO(
                id="order-12345",
                product_id="prod-999",
                price=1500.00,
                status="DELIVERED",
                created_at=datetime.now(timezone.utc),
            ),
            OrderDTO(
                id="order-67890",
                product_id="prod-777",
                price=3200.50,
                status="PENDING",
                created_at=datetime.now(timezone.utc),
            ),
        ]


class MockPaymentsClient(IPaymentsClient):
    async def get_user_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        # Возвращаем фейковый баланс
        return BalanceDTO(amount=5000.00)
