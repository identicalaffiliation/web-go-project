import uuid
from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient
from sqlalchemy.ext.asyncio import async_sessionmaker, create_async_engine
from sqlalchemy.pool import NullPool

from apps.profile.main import app
from internal.adapters.http.dependencies import get_profile_service
from internal.adapters.postgres.models import Base
from internal.adapters.postgres.repository import SqlAlchemyProfileRepository
from internal.adapters.postgres.uow import SqlAlchemyUnitOfWork
from internal.app.service import ProfileService

TEST_DATABASE_URL = (
    "postgresql+asyncpg://postgres:postgres@127.0.0.1:5440/test_profile_db"
)


@pytest.fixture
async def test_db_setup():
    engine = create_async_engine(TEST_DATABASE_URL, echo=False, poolclass=NullPool)
    maker = async_sessionmaker(engine, expire_on_commit=False)

    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)

    yield maker

    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.drop_all)
    await engine.dispose()


@pytest.fixture
def override_service_for_e2e(test_db_setup):
    maker = test_db_setup

    async def _test_build_profile_service():
        async with maker() as session:
            repo = SqlAlchemyProfileRepository(session)
            uow = SqlAlchemyUnitOfWork(session)

            service = ProfileService(
                repo=repo,
                uow=uow,
                orders_client=AsyncMock(),
                payments_client=AsyncMock(),
            )
            yield service

    app.dependency_overrides[get_profile_service] = _test_build_profile_service
    yield
    app.dependency_overrides.clear()


@pytest.mark.asyncio
async def test_e2e_create_and_get_business_profile(override_service_for_e2e):
    user_id = str(uuid.uuid4())

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        post_response = await ac.post(
            "/api/v1/profiles/business",
            json={"company_name": "ООО Интеграция", "phone": "+79998887766"},
            headers={"X-User-Id": user_id},
        )

    assert post_response.status_code == 201

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        get_response = await ac.get(
            "/api/v1/profiles/me", headers={"X-User-Id": user_id}
        )

    assert get_response.status_code == 200
    profiles = get_response.json()

    assert len(profiles) == 1
    assert profiles[0]["company_name"] == "ООО Интеграция"
    assert profiles[0]["profile_type"] == "business"
