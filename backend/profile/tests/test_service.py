import uuid
from unittest.mock import AsyncMock

import pytest

from internal.adapters.http.schemas import CreateBusinessProfileRequest
from internal.app.service import ProfileService
from internal.domain.exceptions import ProfileAlreadyExistsError
from internal.domain.models import Profile, ProfileType


@pytest.fixture
def mock_service():
    """
    Фикстура (Arrange): создает экземпляр ProfileService,
    заменяя все внешние зависимости на AsyncMock (асинхронные заглушки).
    """
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
    # 1. Arrange (Подготовка)
    service, repo_mock, uow_mock = mock_service
    user_id = uuid.uuid4()
    request_data = CreateBusinessProfileRequest(
        company_name="ООО Ромашка", phone="+79991234567"
    )

    # Имитируем, что бизнес-профиля в базе еще нет (репозиторий возвращает None)
    repo_mock.get_by_user_id_and_type.return_value = None

    # Имитируем успешное создание профиля в базе
    expected_profile = Profile(
        id=uuid.uuid4(),
        user_id=user_id,
        profile_type=ProfileType.BUSINESS,
        company_name="ООО Ромашка",
        phone="+79991234567",
    )
    repo_mock.create.return_value = expected_profile

    # 2. Act (Действие)
    result = await service.create_business_profile(user_id=user_id, data=request_data)

    # 3. Assert (Проверка)
    assert result.company_name == "ООО Ромашка"
    assert result.profile_type == ProfileType.BUSINESS

    # Проверяем, что нужные методы были вызваны
    repo_mock.get_by_user_id_and_type.assert_called_once_with(
        user_id=user_id, profile_type=ProfileType.BUSINESS
    )
    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_called_once()  # Важно: транзакция должна быть зафиксирована


@pytest.mark.asyncio
async def test_create_business_profile_already_exists_raises_error(mock_service):
    # 1. Arrange
    service, repo_mock, uow_mock = mock_service
    user_id = uuid.uuid4()
    request_data = CreateBusinessProfileRequest(company_name="Новая Компания")

    # Имитируем ситуацию: в базе уже есть бизнес-профиль для этого user_id
    existing_profile = Profile(
        id=uuid.uuid4(), user_id=user_id, profile_type=ProfileType.BUSINESS
    )
    repo_mock.get_by_user_id_and_type.return_value = existing_profile

    # 2 & 3. Act & Assert
    # Проверяем, что сервис выбросит наше доменное исключение
    with pytest.raises(ProfileAlreadyExistsError) as exc_info:
        await service.create_business_profile(user_id=user_id, data=request_data)

    assert "У данного пользователя уже есть бизнес-профиль" in str(exc_info.value)

    # Убеждаемся, что мы НЕ пытались сохранить дубликат и НЕ сделали commit
    repo_mock.create.assert_not_called()
    uow_mock.commit.assert_not_called()
