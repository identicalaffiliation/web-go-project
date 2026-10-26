import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

import grpc
from fastapi import FastAPI

from internal.adapters.http.dependencies import get_profile_service
from internal.adapters.http.profiles import router as profiles_router
from internal.adapters.kafka.consumer import KafkaUserRegisteredConsumer
from internal.adapters.kafka.producer import KafkaEventProducer
from internal.adapters.postgres.database import async_session_maker, engine
from internal.adapters.postgres.repository import SqlAlchemyProfileRepository
from internal.adapters.postgres.uow import SqlAlchemyUnitOfWork
from internal.adapters.rpc.grpc_clients import GrpcOrdersClient, GrpcPaymentsClient
from internal.app.service import ProfileService
from internal.config.config import settings

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# Глобальные ссылки — заполняются в lifespan
kafka_consumer: KafkaUserRegisteredConsumer | None = None
orders_channel: grpc.aio.Channel | None = None
payments_channel: grpc.aio.Channel | None = None
grpc_orders_client: GrpcOrdersClient | None = None
grpc_payments_client: GrpcPaymentsClient | None = None


async def build_profile_service() -> AsyncGenerator[ProfileService, None]:
    async with async_session_maker() as session:
        try:
            repo = SqlAlchemyProfileRepository(session)
            uow = SqlAlchemyUnitOfWork(session)
            service = ProfileService(
                repo=repo,
                uow=uow,
                orders_client=grpc_orders_client,
                payments_client=grpc_payments_client,
                event_producer=kafka_producer,
            )
            yield service
        finally:
            await session.close()


async def process_kafka_message(msg_data: dict) -> None:
    async with async_session_maker() as session:
        repo = SqlAlchemyProfileRepository(session)
        uow = SqlAlchemyUnitOfWork(session)
        service = ProfileService(
            repo=repo,
            uow=uow,
            orders_client=grpc_orders_client,
            payments_client=grpc_payments_client,
            event_producer=kafka_producer,
        )
        await service.process_registration_event(msg_data)


@asynccontextmanager
async def lifespan(app: FastAPI):
    global kafka_consumer, orders_channel, payments_channel
    global grpc_orders_client, grpc_payments_client

    logger.info("Starting Profile Service...")

    orders_channel = grpc.aio.insecure_channel(settings.ORDERS_GRPC_HOST)
    payments_channel = grpc.aio.insecure_channel(settings.PAYMENTS_GRPC_HOST)
    grpc_orders_client = GrpcOrdersClient(orders_channel)
    grpc_payments_client = GrpcPaymentsClient(payments_channel)

    kafka_consumer = KafkaUserRegisteredConsumer(
        bootstrap_servers=settings.KAFKA_BOOTSTRAP_SERVERS,
        group_id=settings.KAFKA_CONSUMER_GROUP,
        topic=settings.KAFKA_USER_REGISTERED_TOPIC,
        process_callback=process_kafka_message,
    )
    await kafka_consumer.start()

    yield

    logger.info("Shutting down Profile Service...")
    if kafka_consumer:
        await kafka_consumer.stop()
    await engine.dispose()
    await orders_channel.close()
    await payments_channel.close()


app = FastAPI(lifespan=lifespan)
app.dependency_overrides[get_profile_service] = build_profile_service
app.include_router(profiles_router, prefix="/api/v1")
