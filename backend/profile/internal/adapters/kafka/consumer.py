import asyncio
import json
import logging
from collections.abc import Awaitable, Callable
from typing import Any

from aiokafka import AIOKafkaConsumer
from internal.ports.message_broker import IEventConsumer

logger = logging.getLogger(__name__)


class KafkaUserRegisteredConsumer(IEventConsumer):
    def __init__(
        self,
        bootstrap_servers: str,
        group_id: str,
        topic: str,
        process_callback: Callable[[dict[str, Any]], Awaitable[None]],
    ):
        self.bootstrap_servers = bootstrap_servers
        self.group_id = group_id
        self.topic = topic
        self.process_callback = process_callback
        self.consumer: AIOKafkaConsumer | None = None
        self._consume_task: asyncio.Task | None = None

    async def start(self) -> None:
        self.consumer = AIOKafkaConsumer(
            self.topic,
            bootstrap_servers=self.bootstrap_servers,
            group_id=self.group_id,
            value_deserializer=lambda m: json.loads(m.decode("utf-8")),
            auto_offset_reset="earliest",
            enable_auto_commit=False,
        )
        await self.consumer.start()

        self._consume_task = asyncio.create_task(self._consume_loop())
        logger.info(f"Kafka Consumer started listening to topic: {self.topic}")

    async def stop(self) -> None:
        if self._consume_task:
            self._consume_task.cancel()
            try:
                await self._consume_task
            except asyncio.CancelledError:
                pass

        if self.consumer:
            await self.consumer.stop()
            logger.info("Kafka Consumer stopped.")

    async def _consume_loop(self) -> None:
        try:
            async for msg in self.consumer:
                try:
                    await self.process_callback(msg.value)
                    await self.consumer.commit()

                except Exception as e:
                    logger.error(f"Error processing Kafka message {msg.value}: {e}")

        except asyncio.CancelledError:
            logger.info("Kafka consume loop cancelled (shutdown signal received).")
