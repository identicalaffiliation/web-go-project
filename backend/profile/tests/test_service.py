import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient

from apps.profile.main import app
from internal.adapters.http.dependencies import get_profile_service
from internal.adapters.http.schemas import CreateBusinessProfileRequest
from internal.app.service import ProfileService
from internal.domain.exceptions import ProfileAlreadyExistsError, ProfileNotFoundError
from internal.domain.models import Profile, ProfileType


@pytest.fixture
def mock_service():
    repo = AsyncMock()
    uow = AsyncMock()
    orders_client = AsyncMock()
    payments_client = AsyncMock()

    service = ProfileService(
        repo=repo,
        uow=uow,
        orders_client=orders_client,
        payments_client=payments_client,
    )
    return service, repo, uow


@pytest.mark.asyncio
async def test_create_business_profile_success(mock_service):
    service, repo_mock, uow_mock = mock_service
    user_id = uuid.uuid4()
    request_data = CreateBusinessProfileRequest(
        company_name="ООО Ромашка", phone="+79991234567"
    )

    repo_mock.get_by_user_id_and_type.return_value = None

    expected_profile = Profile(
        id=uuid.uuid4(),
        user_id=user_id,
        profile_type=ProfileType.BUSINESS,
        company_name="ООО Ромашка",
        phone="+79991234567",
    )
    repo_mock.create.return_value = expected_profile

    result = await service.create_business_profile(user_id=user_id, data=request_data)

    assert result.company_name == "ООО Ромашка"
    assert result.profile_type == ProfileType.BUSINESS

    repo_mock.get_by_user_id_and_type.assert_called_once_with(
        user_id=user_id, profile_type=ProfileType.BUSINESS
    )
    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_called_once()  # Важно: транзакция должна быть зафиксирована


@pytest.mark.asyncio
async def test_create_business_profile_already_exists_raises_error(mock_service):
    service, repo_mock, uow_mock = mock_service
    user_id = uuid.uuid4()
    request_data = CreateBusinessProfileRequest(company_name="Новая Компания")

    existing_profile = Profile(
        id=uuid.uuid4(), user_id=user_id, profile_type=ProfileType.BUSINESS
    )
    repo_mock.get_by_user_id_and_type.return_value = existing_profile

    with pytest.raises(ProfileAlreadyExistsError) as exc_info:
        await service.create_business_profile(user_id=user_id, data=request_data)

    assert "У данного пользователя уже есть бизнес-профиль" in str(exc_info.value)

    repo_mock.create.assert_not_called()
    uow_mock.commit.assert_not_called()


@pytest.fixture
def mock_profile_service():
    return AsyncMock()


@pytest.fixture
def override_get_service(mock_profile_service):
    app.dependency_overrides[get_profile_service] = lambda: mock_profile_service
    yield
    app.dependency_overrides.clear()


@pytest.mark.asyncio
async def test_create_business_profile_api_success(
    mock_profile_service, override_get_service
):
    user_id = str(uuid.uuid4())
    now = datetime.now(timezone.utc)  # Создаем фиктивную дату

    mock_profile_service.create_business_profile.return_value = Profile(
        id=uuid.uuid4(),
        user_id=uuid.UUID(user_id),
        profile_type=ProfileType.BUSINESS,
        company_name="ООО Вектор",
        phone="+79991234567",
        created_at=now,  # Добавлено
        updated_at=now,  # Добавлено
    )

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.post(
            "/api/v1/profiles/business",
            json={"company_name": "ООО Вектор", "phone": "+79991234567"},
            headers={"X-User-Id": user_id},
        )

    assert response.status_code == 201
    data = response.json()
    assert data["company_name"] == "ООО Вектор"
    assert data["profile_type"] == "business"
    assert "created_at" in data


@pytest.mark.asyncio
async def test_create_business_profile_api_conflict(
    mock_profile_service, override_get_service
):
    user_id = str(uuid.uuid4())

    mock_profile_service.create_business_profile.side_effect = (
        ProfileAlreadyExistsError("Уже есть")
    )

    # 2. Act
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.post(
            "/api/v1/profiles/business",
            json={"company_name": "ООО Вектор"},
            headers={"X-User-Id": user_id},
        )

    assert response.status_code == 409
    assert response.json()["detail"] == "Уже есть"


@pytest.mark.asyncio
async def test_update_profile_success(mock_service):
    # 1. Arrange (Подготовка)
    service, repo_mock, uow_mock = mock_service
    user_id = uuid.uuid4()
    profile_id = uuid.uuid4()

    existing_profile = Profile(
        id=profile_id,
        user_id=user_id,
        profile_type=ProfileType.CUSTOMER,
        first_name="Иван",
        last_name="Иванов",
        phone="+79991112233",
    )
    repo_mock.get_by_user_id.return_value = [existing_profile]

    repo_mock.update.return_value = existing_profile

    update_data = {"first_name": "Пётр", "last_name": "Петров"}

    updated_profile = await service.update_profile(
        profile_id=profile_id, user_id=user_id, update_data=update_data
    )

    assert updated_profile.first_name == "Пётр"
    assert updated_profile.last_name == "Петров"
    assert updated_profile.phone == "+79991112233"

    repo_mock.get_by_user_id.assert_called_once_with(user_id)
    repo_mock.update.assert_called_once()
    uow_mock.commit.assert_called_once()


@pytest.mark.asyncio
async def test_update_profile_not_found(mock_service):
    service, repo_mock, uow_mock = mock_service
    user_id = uuid.uuid4()
    wrong_profile_id = uuid.uuid4()

    repo_mock.get_by_user_id.return_value = []

    update_data = {"first_name": "Хакер"}

    with pytest.raises(ProfileNotFoundError) as exc_info:
        await service.update_profile(
            profile_id=wrong_profile_id, user_id=user_id, update_data=update_data
        )

    assert str(wrong_profile_id) in str(exc_info.value)

    repo_mock.update.assert_not_called()
    uow_mock.commit.assert_not_called()


@pytest.mark.asyncio
async def test_process_registration_event_success(mock_service):
    # 1. Arrange
    service, repo_mock, uow_mock = mock_service
    user_id = str(uuid.uuid4())
    event_id = str(uuid.uuid4())
    event_data = {"user_id": user_id, "event_id": event_id}

    # Имитируем успешное создание в БД (репозиторий возвращает профиль)
    repo_mock.create.return_value = Profile(
        id=uuid.uuid4(), user_id=uuid.UUID(user_id), profile_type=ProfileType.CUSTOMER
    )

    # 2. Act
    await service.process_registration_event(event_data)

    # 3. Assert
    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_called_once()  # Успешная обработка завершается коммитом


@pytest.mark.asyncio
async def test_process_registration_event_idempotency_duplicate(mock_service):
    # 1. Arrange
    service, repo_mock, uow_mock = mock_service
    user_id = str(uuid.uuid4())
    event_id = str(uuid.uuid4())
    event_data = {"user_id": user_id, "event_id": event_id}

    # Имитируем конфликт уникальности: профиль с таким event_id уже есть,
    # и репозиторий возвращает None (как прописано в методе create репозитория)
    repo_mock.create.return_value = None

    # 2. Act
    await service.process_registration_event(event_data)

    # 3. Assert
    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_not_called()
    uow_mock.rollback.assert_called_once()  # Произошел откат транзакции (дубль пропущен)
