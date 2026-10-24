import uuid

import grpc

from internal.adapters.grpc.generated import (
    payments_pb2,
    payments_pb2_grpc,
)
from internal.adapters.grpc.generated import (
    v1_pb2 as orders_pb2,
)
from internal.adapters.grpc.generated import (
    v1_pb2_grpc as orders_pb2_grpc,
)
from internal.domain.models import BalanceDTO, OrderDTO
from internal.ports.rpc_clients import IOrdersClient, IPaymentsClient


class GrpcOrdersClient(IOrdersClient):
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = orders_pb2_grpc.OrdersServiceStub(channel)

    async def get_user_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        request = orders_pb2.UserOrdersRequest(user_id=str(user_id))
        try:
            response = await self.stub.GetUserOrders(request)

            return [
                OrderDTO(
                    id=order.order_id,
                    product_id=order.product_id,
                    price=float(order.total_price),
                    status=orders_pb2.OrderStatus.Name(order.status),
                    created_at=order.created_at.ToDatetime(),
                )
                for order in response.orders
            ]
        except grpc.aio.AioRpcError as e:
            raise Exception(f"Orders service error: {e.details()}") from None


class GrpcPaymentsClient(IPaymentsClient):
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = payments_pb2_grpc.PaymentsServiceStub(channel)

    async def get_user_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        request = payments_pb2.GetBalanceRequest(user_id=str(user_id))
        try:
            response = await self.stub.GetBalanceByUser(request)
            return BalanceDTO(amount=response.amount)
        except grpc.aio.AioRpcError as e:
            raise Exception(f"Payments service error: {e.details()}") from None
