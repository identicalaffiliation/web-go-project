import asyncio
import uuid
from unittest.mock import AsyncMock, patch

import grpc
import pytest
from google.protobuf.timestamp_pb2 import Timestamp

from internal.adapters.grpc.generated.v1.orders import orders_pb2
from internal.adapters.grpc.generated.v1.payments import payments_pb2
from internal.adapters.rpc.circuit_breaker import (
    CircuitBreaker,
    CircuitBreakerOpenError,
    CircuitBreakerState,
)
from internal.adapters.rpc.grpc_clients import (
    GrpcOrdersClient,
    GrpcPaymentsClient,
    create_balanced_channel,
    execute_with_retry,
)


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


@pytest.mark.asyncio
async def test_retry_on_resource_exhausted_success():
    """Тест: перехват RESOURCE_EXHAUSTED и успешный ответ на второй попытке."""
    call_mock = AsyncMock()

    # Имитируем AioRpcError со статусом RESOURCE_EXHAUSTED
    mock_error = grpc.aio.AioRpcError(
        code=grpc.StatusCode.RESOURCE_EXHAUSTED,
        initial_metadata=None,
        trailing_metadata=None,
        details="Too many requests",
    )
    mock_error.code = lambda: grpc.StatusCode.RESOURCE_EXHAUSTED
    mock_error.details = lambda: "Too many requests"

    call_mock.side_effect = [mock_error, "success_result"]

    result = await execute_with_retry(
        call_mock,
        max_retries=2,
        initial_backoff=0.01,
        multiplier=1.0,
        max_backoff=0.01,
    )

    assert result == "success_result"
    assert call_mock.await_count == 2


@pytest.mark.asyncio
async def test_retry_exhausted_raises_runtime_error():
    """Тест: если все попытки исчерпаны, выбрасывается RuntimeError."""
    call_mock = AsyncMock()
    mock_error = grpc.aio.AioRpcError(
        code=grpc.StatusCode.RESOURCE_EXHAUSTED,
        initial_metadata=None,
        trailing_metadata=None,
        details="Too many requests",
    )
    mock_error.code = lambda: grpc.StatusCode.RESOURCE_EXHAUSTED
    mock_error.details = lambda: "Too many requests"

    call_mock.side_effect = mock_error

    with pytest.raises(RuntimeError) as exc_info:
        await execute_with_retry(
            call_mock,
            max_retries=2,
            initial_backoff=0.01,
            multiplier=1.0,
            max_backoff=0.01,
        )

    assert "RESOURCE_EXHAUSTED" in str(exc_info.value)
    assert call_mock.await_count == 3  # 1 первичный + 2 ретрая


def test_create_balanced_channel_options():
    """Проверка передачи опций round_robin в gRPC канал."""
    channel = create_balanced_channel("orders-service:50051")
    assert channel is not None


@pytest.mark.asyncio
async def test_circuit_breaker_trips_to_open_after_failures():
    """Проверка перехода в OPEN при достижении порога ошибок."""
    cb = CircuitBreaker(name="test_cb", failure_threshold=2, recovery_timeout=0.1)

    async def failing_call():
        raise RuntimeError("Service down")

    with pytest.raises(RuntimeError):
        await cb.call(failing_call)
    assert cb.state == CircuitBreakerState.CLOSED

    with pytest.raises(RuntimeError):
        await cb.call(failing_call)
    assert cb.state == CircuitBreakerState.OPEN

    with pytest.raises(CircuitBreakerOpenError):
        await cb.call(failing_call)


@pytest.mark.asyncio
async def test_circuit_breaker_recovers_to_closed():
    """Проверка восстановления цепи в CLOSED после успешного пробного запроса в HALF_OPEN."""
    cb = CircuitBreaker(name="test_cb", failure_threshold=1, recovery_timeout=0.05)

    async def failing_call():
        raise RuntimeError("Service down")

    with pytest.raises(RuntimeError):
        await cb.call(failing_call)
    assert cb.state == CircuitBreakerState.OPEN

    await asyncio.sleep(0.06)

    async def success_call():
        return "ok"

    result = await cb.call(success_call)
    assert result == "ok"
    assert cb.state == CircuitBreakerState.CLOSED


@pytest.mark.asyncio
async def test_grpc_orders_client_circuit_breaker_blocks_requests():
    """Тест интеграции GrpcOrdersClient и CircuitBreaker: блокировка запросов при сбоях."""
    cb = CircuitBreaker(name="orders_test", failure_threshold=1, recovery_timeout=1.0)
    stub_mock = AsyncMock()
    stub_mock.GetUserOrders.side_effect = RuntimeError("Service Unavailable")

    with patch(
        "internal.adapters.rpc.grpc_clients.orders_pb2_grpc.OrdersServiceStub",
        return_value=stub_mock,
    ):
        client = GrpcOrdersClient(channel=AsyncMock(), circuit_breaker=cb)

        with pytest.raises(RuntimeError):
            await client.get_user_orders(uuid.uuid4())

        assert cb.state == CircuitBreakerState.OPEN

        with pytest.raises(CircuitBreakerOpenError):
            await client.get_user_orders(uuid.uuid4())

        assert stub_mock.GetUserOrders.await_count == 1


async def test_grpc_payments_client_circuit_breaker_blocks_requests():
    """GrpcPaymentsClient моментально отклоняет последующие запросы при размыкании цепи."""
    cb = CircuitBreaker(name="payments_test", failure_threshold=1, recovery_timeout=1.0)
    stub_mock = AsyncMock()
    stub_mock.GetBalanceByUser.side_effect = RuntimeError("Payments down")

    with patch(
        "internal.adapters.rpc.grpc_clients.payments_pb2_grpc.PaymentsServiceStub",
        return_value=stub_mock,
    ):
        client = GrpcPaymentsClient(channel=AsyncMock(), circuit_breaker=cb)

        with pytest.raises(RuntimeError):
            await client.get_user_balance(uuid.uuid4())

        assert cb.state == CircuitBreakerState.OPEN

        with pytest.raises(CircuitBreakerOpenError):
            await client.get_user_balance(uuid.uuid4())

        assert stub_mock.GetBalanceByUser.await_count == 1
