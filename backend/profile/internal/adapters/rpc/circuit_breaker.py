import asyncio
import logging
import time
from collections.abc import Awaitable, Callable
from enum import Enum
from typing import TypeVar

logger = logging.getLogger(__name__)

T = TypeVar("T")


class CircuitBreakerState(str, Enum):
    CLOSED = "CLOSED"
    OPEN = "OPEN"
    HALF_OPEN = "HALF_OPEN"


class CircuitBreakerOpenError(RuntimeError):
    """Выбрасывается при попытке вызова, когда цепь разомкнута."""


class CircuitBreaker:
    def __init__(
        self,
        name: str = "default",
        failure_threshold: int = 3,
        recovery_timeout: float = 10.0,
    ):
        self.name = name
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout

        self._state = CircuitBreakerState.CLOSED
        self._failure_count = 0
        self._last_state_change = time.monotonic()
        self._lock = asyncio.Lock()

    @property
    def state(self) -> CircuitBreakerState:
        return self._state

    async def call(self, func: Callable[[], Awaitable[T]]) -> T:
        async with self._lock:
            now = time.monotonic()
            if self._state == CircuitBreakerState.OPEN:
                if now - self._last_state_change >= self.recovery_timeout:
                    logger.info(
                        "Circuit breaker [%s] transitioning from OPEN to HALF_OPEN",
                        self.name,
                    )
                    self._state = CircuitBreakerState.HALF_OPEN
                else:
                    raise CircuitBreakerOpenError(
                        f"Circuit breaker [{self.name}] is OPEN. Call rejected."
                    )

        try:
            result = await func()
            await self._on_success()
            return result
        except Exception as e:
            if isinstance(e, CircuitBreakerOpenError):
                raise
            await self._on_failure()
            raise

    async def _on_success(self) -> None:
        async with self._lock:
            if self._state != CircuitBreakerState.CLOSED:
                logger.info(
                    "Circuit breaker [%s] recovered! Transitioning to CLOSED",
                    self.name,
                )
            self._state = CircuitBreakerState.CLOSED
            self._failure_count = 0

    async def _on_failure(self) -> None:
        async with self._lock:
            self._failure_count += 1
            now = time.monotonic()
            if (
                self._state == CircuitBreakerState.HALF_OPEN
                or self._failure_count >= self.failure_threshold
            ):
                logger.warning(
                    "Circuit breaker [%s] tripped to OPEN (failures: %d)",
                    self.name,
                    self._failure_count,
                )
                self._state = CircuitBreakerState.OPEN
                self._last_state_change = now
