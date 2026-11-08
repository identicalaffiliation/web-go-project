import uuid
from unittest.mock import AsyncMock

import pytest

from internal.adapters.kafka.consumer import KafkaLikesConsumer


@pytest.mark.asyncio
async def test_kafka_consumer_commits_on_success():
    callback = AsyncMock()
    consumer = KafkaLikesConsumer(
        bootstrap_servers="mock:9092",
        group_id="test_group",
        topic="like.events",
        process_callback=callback,
    )
    fake_msg = AsyncMock(
        value={
            "user_id": str(uuid.uuid4()),
            "product_id": str(uuid.uuid4()),
            "action": "add",
        }
    )

    consumer.consumer = AsyncMock()
    await callback(fake_msg.value)
    await consumer.consumer.commit()

    callback.assert_awaited_once_with(fake_msg.value)
    consumer.consumer.commit.assert_awaited_once()
