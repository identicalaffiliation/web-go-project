import uuid
from unittest.mock import AsyncMock, patch

import pytest
from google.protobuf.timestamp_pb2 import Timestamp

from internal.adapters.grpc.generated.v1.orders import orders_pb2
from internal.adapters.grpc.generated.v1.payments import payments_pb2
from internal.adapters.rpc.grpc_clients import GrpcOrdersClient, GrpcPaymentsClient


@pytest.mark.asyncio
async def test_grpc_orders_client_success():
    with patch(
        "internal.adapters.rpc.grpc_clients.orders_pb2_grpc.OrdersServiceStub"
    ) as StubMock:
        stub = StubMock.return_value
        stub.GetUserOrders = AsyncMock()

        ts = Timestamp()
        ts.GetCurrentTime()
        stub.GetUserOrders.return_value = orders_pb2.UserOrdersResponse(
            orders=[
                orders_pb2.UserOrderResponse(
                    order_id="ord-123",
                    product_id="prod-999",
                    user_id="user-1",
                    quantity=2,
                    total_price=300,
                    status=orders_pb2.CREATED,
                    created_at=ts,
                    updated_at=ts,
                )
            ]
        )

        client = GrpcOrdersClient(channel=AsyncMock())
        orders = await client.get_user_orders(uuid.uuid4())

        assert len(orders) == 1
        assert orders[0].id == "ord-123"
        assert orders[0].total_price == 300
        assert orders[0].quantity == 2
        assert orders[0].status == "CREATED"
        stub.GetUserOrders.assert_awaited_once()


@pytest.mark.asyncio
async def test_grpc_payments_client_success():
    with patch(
        "internal.adapters.rpc.grpc_clients.payments_pb2_grpc.PaymentsServiceStub"
    ) as StubMock:
        stub = StubMock.return_value
        stub.GetBalanceByUser = AsyncMock()
        stub.GetBalanceByUser.return_value = payments_pb2.GetBalanceResponse(
            user_id="user-1", amount=100_500
        )

        client = GrpcPaymentsClient(channel=AsyncMock())
        balance = await client.get_user_balance(uuid.uuid4())

        assert balance.amount == 100_500
