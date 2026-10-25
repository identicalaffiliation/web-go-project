import json
import logging
from typing import Any

from aiokafka import AIOKafkaProducer

logger = logging.getLogger(__name__)


class KafkaEventProducer:
    def __init__(self, bootstrap_servers: str):
        self._producer = AIOKafkaProducer(
            bootstrap_servers=bootstrap_servers,
            value_serializer=lambda v: json.dumps(v).encode("utf-8"),
        )

    async def start(self) -> None:
        await self._producer.start()
        logger.info("Kafka Producer started successfully")

    async def stop(self) -> None:
        await self._producer.stop()
        logger.info("Kafka Producer stopped")

    async def publish_event(
        self, topic: str, key: str, payload: dict[str, Any]
    ) -> None:
        """
        Отправляет событие с ожиданием подтверждения от брокера (ack).
        Использует user_id в качестве ключа партиционирования.
        """
        key_bytes = key.encode("utf-8")
        try:
            await self._producer.send_and_wait(
                topic=topic,
                key=key_bytes,
                value=payload,
            )
            logger.info(
                "Published event %s to topic %s for key %s",
                payload.get("event_type"),
                topic,
                key,
            )
        except Exception:
            logger.exception("Failed to publish event to topic %s", topic)
            raise
