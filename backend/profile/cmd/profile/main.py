import logging
from collections.abc import AsyncGenerator
from contextlib import asynccontextmanager

from fastapi import FastAPI
from internal.adapters.http.dependencies import get_profile_service
from internal.adapters.http.profiles import router as profiles_router
from internal.adapters.kafka.consumer import KafkaUserRegisteredConsumer
from internal.adapters.postgres.database import async_session_maker, engine
from internal.adapters.postgres.repository import SqlAlchemyProfileRepository
from internal.adapters.postgres.uow import SqlAlchemyUnitOfWork
from internal.adapters.rpc.mock_clients import MockOrdersClient, MockPaymentsClient
from internal.app.service import ProfileService
from internal.config.config import settings

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

kafka_consumer: KafkaUserRegisteredConsumer | None = None


async def build_profile_service() -> AsyncGenerator[ProfileService, None]:
    async with async_session_maker() as session:
        repo = SqlAlchemyProfileRepository(session)
        uow = SqlAlchemyUnitOfWork(session)
        orders_client = MockOrdersClient()
        payments_client = MockPaymentsClient()

        service = ProfileService(
            repo=repo,
            uow=uow,
            orders_client=orders_client,
            payments_client=payments_client,
        )
        yield service


async def process_kafka_message(msg_data: dict) -> None:
    async with async_session_maker() as session:
        repo = SqlAlchemyProfileRepository(session)
        uow = SqlAlchemyUnitOfWork(session)
        orders_client = MockOrdersClient()
        payments_client = MockPaymentsClient()

        service = ProfileService(
            repo=repo,
            uow=uow,
            orders_client=orders_client,
            payments_client=payments_client,
        )
        await service.process_registration_event(msg_data)


@asynccontextmanager
async def lifespan(app: FastAPI):
    global kafka_consumer

    logger.info("Starting Profile Service...")

    kafka_consumer = KafkaUserRegisteredConsumer(
        bootstrap_servers=settings.KAFKA_BOOTSTRAP_SERVERS,
        group_id=settings.KAFKA_CONSUMER_GROUP,
        topic="auth.user.registered",
        process_callback=process_kafka_message,
    )

    await kafka_consumer.start()

    yield

    logger.info("Shutting down Profile Service...")

    if kafka_consumer:
        await kafka_consumer.stop()

    await engine.dispose()


app = FastAPI(
    title="Profile Service",
    description="Микросервис управления профилями",
    version="1.0.0",
    lifespan=lifespan,
)

app.dependency_overrides[get_profile_service] = build_profile_service

app.include_router(profiles_router, prefix="/api/v1")

if __name__ == "__main__":
    import uvicorn

    uvicorn.run("cmd.profile.main:app", host="127.0.0.1", port=8000, reload=True)
