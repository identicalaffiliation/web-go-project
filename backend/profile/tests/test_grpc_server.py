import asyncio
import uuid
from unittest.mock import patch

import grpc
import pytest
from google.protobuf.timestamp_pb2 import Timestamp

from internal.adapters.grpc.generated.v1.orders import (
    orders_pb2,
    orders_pb2_grpc,
)
from internal.adapters.grpc.generated.v1.payments import (
    payments_pb2,
    payments_pb2_grpc,
)
from internal.adapters.rpc.grpc_clients import GrpcOrdersClient, GrpcPaymentsClient


class FakeOrdersServicer(orders_pb2_grpc.OrdersServiceServicer):
    async def GetUserOrders(self, request, context):
        if not request.user_id:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id required")

        ts = Timestamp()
        ts.GetCurrentTime()
        return orders_pb2.UserOrdersResponse(
            orders=[
                orders_pb2.UserOrderResponse(
                    order_id="ord-1",
                    product_id="prod-1",
                    user_id=request.user_id,
                    quantity=2,
                    total_price=500,
                    status=orders_pb2.CREATED,
                    created_at=ts,
                    updated_at=ts,
                )
            ]
        )


class FakePaymentsServicer(payments_pb2_grpc.PaymentsServiceServicer):
    async def GetBalanceByUser(self, request, context):
        if not request.user_id:
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "user_id required")
        return payments_pb2.GetBalanceResponse(user_id=request.user_id, amount=100_500)


class SlowOrdersServicer(orders_pb2_grpc.OrdersServiceServicer):
    async def GetUserOrders(self, request, context):
        await asyncio.sleep(10)
        return orders_pb2.UserOrdersResponse()


class FailingOrdersServicer(orders_pb2_grpc.OrdersServiceServicer):
    async def GetUserOrders(self, request, context):
        await context.abort(grpc.StatusCode.INTERNAL, "boom")


@pytest.fixture
async def orders_server():
    server = grpc.aio.server()
    orders_pb2_grpc.add_OrdersServiceServicer_to_server(FakeOrdersServicer(), server)
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    yield f"127.0.0.1:{port}"
    await server.stop(grace=None)


@pytest.fixture
async def payments_server():
    server = grpc.aio.server()
    payments_pb2_grpc.add_PaymentsServiceServicer_to_server(
        FakePaymentsServicer(), server
    )
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    yield f"127.0.0.1:{port}"
    await server.stop(grace=None)


@pytest.mark.asyncio
async def test_orders_client_against_real_server(orders_server):
    """Реальный gRPC-сервер, реальный TCP, реальный wire."""
    async with grpc.aio.insecure_channel(orders_server) as channel:
        client = GrpcOrdersClient(channel)
        user_id = uuid.uuid4()

        orders = await client.get_user_orders(user_id)

    assert len(orders) == 1
    o = orders[0]
    assert o.id == "ord-1"
    assert o.product_id == "prod-1"
    assert o.quantity == 2
    assert o.total_price == 500
    assert o.status == "CREATED"
    assert o.created_at is not None


@pytest.mark.asyncio
async def test_payments_client_against_real_server(payments_server):
    async with grpc.aio.insecure_channel(payments_server) as channel:
        client = GrpcPaymentsClient(channel)
        balance = await client.get_user_balance(uuid.uuid4())

    assert balance.amount == 100_500


@pytest.mark.asyncio
async def test_orders_client_timeout():
    server = grpc.aio.server()
    orders_pb2_grpc.add_OrdersServiceServicer_to_server(SlowOrdersServicer(), server)
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    try:
        async with grpc.aio.insecure_channel(f"127.0.0.1:{port}") as channel:
            client = GrpcOrdersClient(channel)
            with patch("internal.adapters.rpc.grpc_clients.GRPC_TIMEOUT", 0.5):
                with pytest.raises(RuntimeError) as exc:
                    await client.get_user_orders(uuid.uuid4())
                assert (
                    "Deadline" in str(exc.value) or "deadline" in str(exc.value).lower()
                )
    finally:
        await server.stop(grace=None)


@pytest.mark.asyncio
async def test_orders_client_server_error():
    server = grpc.aio.server()
    orders_pb2_grpc.add_OrdersServiceServicer_to_server(FailingOrdersServicer(), server)
    port = server.add_insecure_port("127.0.0.1:0")
    await server.start()
    try:
        async with grpc.aio.insecure_channel(f"127.0.0.1:{port}") as channel:
            client = GrpcOrdersClient(channel)
            with pytest.raises(RuntimeError) as exc:
                await client.get_user_orders(uuid.uuid4())
            assert "boom" in str(exc.value)
    finally:
        await server.stop(grace=None)
