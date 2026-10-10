import uuid

import grpc

from internal.adapters.grpc.generated import (
    orders_pb2,
    orders_pb2_grpc,
    payments_pb2,
    payments_pb2_grpc,
)


class GrpcOrdersClient:
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = orders_pb2_grpc.OrdersServiceStub(channel)

    async def get_orders(self, user_id: uuid.UUID):
        request = orders_pb2.GetOrdersRequest(user_id=str(user_id))
        try:
            response = await self.stub.GetOrdersByUser(request)

            return [
                {
                    "id": order.id,
                    "product_id": order.product_id,
                    "price": order.price,
                    "status": order.status,
                    "created_at": order.created_at.ToDatetime(),
                }
                for order in response.orders
            ]
        except grpc.aio.AioRpcError as e:
            raise Exception(f"Orders service error: {e.details()}") from None


class GrpcPaymentsClient:
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = payments_pb2_grpc.PaymentsServiceStub(channel)

    async def get_user_balance(self, user_id: uuid.UUID) -> float:
        request = payments_pb2.GetBalanceRequest(user_id=str(user_id))
        try:
            response = await self.stub.GetBalanceByUser(request)
            return response.amount
        except grpc.aio.AioRpcError as e:
            raise Exception(f"Payments service error: {e.details()}") from None
