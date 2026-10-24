import uuid
from unittest.mock import AsyncMock, patch

import pytest
from google.protobuf.timestamp_pb2 import Timestamp

from internal.adapters.grpc.generated import orders_pb2, payments_pb2
from internal.adapters.rpc.grpc_clients import GrpcOrdersClient, GrpcPaymentsClient


@pytest.mark.asyncio
async def test_grpc_orders_client_success():
    with patch(
        "internal.adapters.rpc.grpc_clients.orders_pb2_grpc.OrdersServiceStub"
    ) as StubMock:
        mock_stub_instance = StubMock.return_value
        mock_stub_instance.GetOrdersByUser = AsyncMock()

        ts = Timestamp()
        ts.GetCurrentTime()
        fake_response = orders_pb2.GetOrdersResponse(
            orders=[
                orders_pb2.OrderItem(
                    id="ord-123",
                    product_id="prod-999",
                    price=150.0,
                    status="COMPLETED",
                    created_at=ts,
                )
            ]
        )
        mock_stub_instance.GetOrdersByUser.return_value = fake_response

        client = GrpcOrdersClient(channel=AsyncMock())
        user_id = uuid.uuid4()

        orders = await client.get_user_orders(user_id)

        assert len(orders) == 1
        assert orders[0].id == "ord-123"
        assert orders[0].price == 150.0
        assert orders[0].status == "COMPLETED"

        called_request = mock_stub_instance.GetOrdersByUser.call_args[0][0]
        assert called_request.user_id == str(user_id)


@pytest.mark.asyncio
async def test_grpc_payments_client_success():
    with patch(
        "internal.adapters.rpc.grpc_clients.payments_pb2_grpc.PaymentsServiceStub"
    ) as StubMock:
        mock_stub_instance = StubMock.return_value
        mock_stub_instance.GetBalanceByUser = AsyncMock()

        fake_response = payments_pb2.GetBalanceResponse(
            user_id="user-1", amount=100500.50
        )
        mock_stub_instance.GetBalanceByUser.return_value = fake_response

        client = GrpcPaymentsClient(channel=AsyncMock())
        user_id = uuid.uuid4()

        balance = await client.get_user_balance(user_id)

        assert balance.amount == 100500.50

        called_request = mock_stub_instance.GetBalanceByUser.call_args[0][0]
        assert called_request.user_id == str(user_id)
