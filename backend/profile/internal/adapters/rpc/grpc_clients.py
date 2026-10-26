import logging
import uuid

import grpc

from internal.adapters.grpc.generated.v1.orders import (
    orders_pb2,
    orders_pb2_grpc,
)
from internal.adapters.grpc.generated.v1.payments import (
    payments_pb2,
    payments_pb2_grpc,
)
from internal.domain.models import BalanceDTO, OrderDTO

logger = logging.getLogger(__name__)
GRPC_TIMEOUT = 5.0


class GrpcOrdersClient:
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = orders_pb2_grpc.OrdersServiceStub(channel)

    async def get_user_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        request = orders_pb2.UserOrdersRequest(user_id=str(user_id))
        try:
            response = await self.stub.GetUserOrders(request, timeout=GRPC_TIMEOUT)
            return [
                OrderDTO(
                    id=o.order_id,
                    product_id=o.product_id,
                    quantity=o.quantity,
                    total_price=o.total_price,
                    status=orders_pb2.OrderStatus.Name(o.status),
                    created_at=o.created_at.ToDatetime(),
                )
                for o in response.orders
            ]
        except grpc.aio.AioRpcError as e:
            logger.error("Orders RPC failed: %s %s", e.code(), e.details())
            raise RuntimeError(f"Orders service error: {e.details()}") from e


class GrpcPaymentsClient:
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = payments_pb2_grpc.PaymentsServiceStub(channel)

    async def get_user_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        request = payments_pb2.GetBalanceRequest(user_id=str(user_id))
        try:
            response = await self.stub.GetBalanceByUser(request, timeout=GRPC_TIMEOUT)
            return BalanceDTO(amount=response.amount)
        except grpc.aio.AioRpcError as e:
            logger.error("Payments RPC failed: %s %s", e.code(), e.details())
            raise RuntimeError(f"Payments service error: {e.details()}") from e
