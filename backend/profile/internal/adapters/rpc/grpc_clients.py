import asyncio
import json
import logging
import uuid
from collections.abc import Awaitable, Callable
from typing import TypeVar

import grpc

from internal.adapters.grpc.generated.v1.orders import (
    orders_pb2,
    orders_pb2_grpc,
)
from internal.adapters.grpc.generated.v1.payments import (
    payments_pb2,
    payments_pb2_grpc,
)
from internal.adapters.rpc.circuit_breaker import CircuitBreaker
from internal.config.config import settings
from internal.domain.models import BalanceDTO, OrderDTO

logger = logging.getLogger(__name__)
GRPC_TIMEOUT = 5.0

T = TypeVar("T")

RETRYABLE_STATUS_CODES = {
    grpc.StatusCode.RESOURCE_EXHAUSTED,
    grpc.StatusCode.UNAVAILABLE,
}


def create_balanced_channel(target: str) -> grpc.aio.Channel:
    """
    Создает gRPC канал с клиентской балансировкой Round Robin.
    Если передан хост без схемы, добавляется префикс dns:///.
    """
    service_config = json.dumps({"loadBalancingConfig": [{"round_robin": {}}]})

    options = [
        ("grpc.service_config", service_config),
        ("grpc.enable_retries", 0),
    ]

    channel_target = target
    if not (target.startswith(("dns://", "ipv4:", "ipv6:"))):
        channel_target = f"dns:///{target}"

    return grpc.aio.insecure_channel(channel_target, options=options)


async def execute_with_retry(
    rpc_call: Callable[[], Awaitable[T]],
    max_retries: int | None = None,
    initial_backoff: float | None = None,
    multiplier: float | None = None,
    max_backoff: float | None = None,
) -> T:
    """
    Выполняет асинхронный gRPC вызов с экспоненциальным backoff при перегрузке (RESOURCE_EXHAUSTED).
    """
    retries = max_retries if max_retries is not None else settings.GRPC_MAX_RETRIES
    backoff = (
        initial_backoff
        if initial_backoff is not None
        else settings.GRPC_INITIAL_BACKOFF_SEC
    )
    mult = multiplier if multiplier is not None else settings.GRPC_BACKOFF_MULTIPLIER
    max_b = max_backoff if max_backoff is not None else settings.GRPC_MAX_BACKOFF_SEC

    attempt = 0
    while True:
        try:
            return await rpc_call()
        except grpc.aio.AioRpcError as e:
            attempt += 1
            if e.code() in RETRYABLE_STATUS_CODES and attempt <= retries:
                logger.warning(
                    "gRPC error %s (%s). Retrying attempt %d/%d after %.2fs...",
                    e.code(),
                    e.details(),
                    attempt,
                    retries,
                    backoff,
                )
                await asyncio.sleep(backoff)
                backoff = min(backoff * mult, max_b)
                continue

            logger.error("gRPC call failed permanently: %s %s", e.code(), e.details())
            raise RuntimeError(
                f"RPC service error [{e.code().name}]: {e.details()}"
            ) from e


class GrpcOrdersClient:
    def __init__(
        self,
        channel: grpc.aio.Channel,
        circuit_breaker: CircuitBreaker | None = None,
    ):
        self.stub = orders_pb2_grpc.OrdersServiceStub(channel)
        self.circuit_breaker = circuit_breaker or CircuitBreaker(
            name="orders_service",
            failure_threshold=settings.CIRCUIT_BREAKER_FAILURE_THRESHOLD,
            recovery_timeout=settings.CIRCUIT_BREAKER_RECOVERY_TIMEOUT_SEC,
        )

    async def get_user_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        request = orders_pb2.UserOrdersRequest(user_id=str(user_id))

        async def _call() -> orders_pb2.UserOrdersResponse:
            return await self.stub.GetUserOrders(request, timeout=GRPC_TIMEOUT)

        response = await self.circuit_breaker.call(lambda: execute_with_retry(_call))

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


class GrpcPaymentsClient:
    def __init__(self, channel: grpc.aio.Channel):
        self.stub = payments_pb2_grpc.PaymentsServiceStub(channel)

    async def get_user_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        request = payments_pb2.GetBalanceRequest(user_id=str(user_id))

        async def _call() -> payments_pb2.GetBalanceResponse:
            return await self.stub.GetBalanceByUser(request, timeout=GRPC_TIMEOUT)

        response = await execute_with_retry(_call)
        return BalanceDTO(amount=response.amount)
