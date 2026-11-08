import uuid
from datetime import datetime, timezone
from unittest.mock import AsyncMock

import pytest
from httpx import ASGITransport, AsyncClient

from apps.profile.main import app
from internal.adapters.http.schemas import CreateBusinessProfileRequest
from internal.app.service import ProfileService
from internal.domain.exceptions import ProfileAlreadyExistsError, ProfileNotFoundError
from internal.domain.models import (
    BusinessProductDTO,
    BusinessProductsPageDTO,
    Profile,
    ProfileType,
)


@pytest.fixture
def mock_service():
    repo = AsyncMock()
    uow = AsyncMock()
    orders_client = AsyncMock()
    payments_client = AsyncMock()
    catalog_client = AsyncMock()
    event_producer = AsyncMock()
    cache = AsyncMock()

    service = ProfileService(
        repo=repo,
        uow=uow,
        orders_client=orders_client,
        payments_client=payments_client,
        catalog_client=catalog_client,
        event_producer=event_producer,
        cache=cache,
    )
    return service, repo, uow, event_producer, cache


@pytest.mark.asyncio
async def test_create_business_profile_success(mock_service):
    service, repo_mock, uow_mock, producer_mock, cache_mock = mock_service
    user_id = uuid.uuid4()
    request_data = CreateBusinessProfileRequest(
        company_name="ООО Ромashka", phone="+79991234567"
    )

    repo_mock.get_by_user_id_and_type.return_value = None

    expected_profile = Profile(
        id=uuid.uuid4(),
        user_id=user_id,
        profile_type=ProfileType.BUSINESS,
        company_name="ООО Ромashka",
        phone="+79991234567",
    )
    repo_mock.create.return_value = expected_profile

    result = await service.create_business_profile(user_id=user_id, data=request_data)

    assert result.company_name == "ООО Ромashka"
    assert result.profile_type == ProfileType.BUSINESS

    repo_mock.get_by_user_id_and_type.assert_called_once_with(
        user_id=user_id, profile_type=ProfileType.BUSINESS
    )
    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_called_once()

    producer_mock.publish_event.assert_called_once()
    call_args = producer_mock.publish_event.call_args[1]

    assert call_args["topic"] == "profiles.business.created"
    assert call_args["key"] == str(user_id)
    assert call_args["payload"]["user_id"] == str(user_id)
    assert "event_id" in call_args["payload"]
    assert "occurred_at" in call_args["payload"]


@pytest.mark.asyncio
async def test_create_business_profile_already_exists_raises_error(mock_service):
    service, repo_mock, uow_mock, producer_mock, cache_mock = mock_service
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
    producer_mock.publish_event.assert_not_called()


@pytest.mark.asyncio
async def test_create_business_profile_api_success(
    mock_profile_service, override_get_service
):
    user_id = str(uuid.uuid4())
    now = datetime.now(timezone.utc)

    mock_profile_service.create_business_profile.return_value = Profile(
        id=uuid.uuid4(),
        user_id=uuid.UUID(user_id),
        profile_type=ProfileType.BUSINESS,
        company_name="ООО Вектор",
        phone="+79991234567",
        created_at=now,
        updated_at=now,
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
    service, repo_mock, uow_mock, producer_mock, cache_mock = mock_service
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
    producer_mock.publish_event.assert_not_called()


@pytest.mark.asyncio
async def test_update_profile_not_found(mock_service):
    service, repo_mock, uow_mock, producer_mock, cache_mock = mock_service
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
    producer_mock.publish_event.assert_not_called()


@pytest.mark.asyncio
async def test_process_registration_event_success(mock_service):
    service, repo_mock, uow_mock, producer_mock, cache_mock = mock_service
    user_id = str(uuid.uuid4())
    event_id = str(uuid.uuid4())
    event_data = {"user_id": user_id, "event_id": event_id}

    repo_mock.create.return_value = Profile(
        id=uuid.uuid4(), user_id=uuid.UUID(user_id), profile_type=ProfileType.CUSTOMER
    )

    await service.process_registration_event(event_data)

    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_called_once()
    producer_mock.publish_event.assert_not_called()


@pytest.mark.asyncio
async def test_process_registration_event_idempotency_duplicate(mock_service):
    service, repo_mock, uow_mock, producer_mock, cache_mock = mock_service
    user_id = str(uuid.uuid4())
    event_id = str(uuid.uuid4())
    event_data = {"user_id": user_id, "event_id": event_id}

    repo_mock.create.return_value = None

    await service.process_registration_event(event_data)

    repo_mock.create.assert_called_once()
    uow_mock.commit.assert_not_called()
    uow_mock.rollback.assert_called_once()
    producer_mock.publish_event.assert_not_called()


@pytest.mark.asyncio
async def test_get_profiles_cache_hit(mock_service):
    service, repo_mock, _, _, cache_mock = mock_service
    user_id = uuid.uuid4()
    profile_id = uuid.uuid4()

    cached_json = (
        f'[{{"id": "{profile_id}", "user_id": "{user_id}", '
        f'"profile_type": "customer", "first_name": "Иван", '
        f'"last_name": null, "phone": null, "company_name": null, '
        f'"created_at": null, "updated_at": null}}]'
    )
    cache_mock.get.return_value = cached_json

    result = await service.get_profiles_by_user(user_id)

    assert len(result) == 1
    assert result[0].id == profile_id
    assert result[0].first_name == "Иван"
    cache_mock.get.assert_called_once_with(f"profiles:{user_id}")
    repo_mock.get_by_user_id.assert_not_called()


@pytest.mark.asyncio
async def test_get_profiles_cache_miss(mock_service):
    service, repo_mock, _, _, cache_mock = mock_service
    user_id = uuid.uuid4()
    profile = Profile(
        id=uuid.uuid4(),
        user_id=user_id,
        profile_type=ProfileType.CUSTOMER,
        first_name="Ольга",
    )

    cache_mock.get.return_value = None
    repo_mock.get_by_user_id.return_value = [profile]

    result = await service.get_profiles_by_user(user_id)

    assert len(result) == 1
    assert result[0].first_name == "Ольга"
    cache_mock.get.assert_called_once_with(f"profiles:{user_id}")
    repo_mock.get_by_user_id.assert_called_once_with(user_id)
    cache_mock.set.assert_called_once()
    assert cache_mock.set.call_args[0][0] == f"profiles:{user_id}"


@pytest.mark.asyncio
async def test_update_profile_invalidates_cache(mock_service):
    service, repo_mock, uow_mock, _, cache_mock = mock_service
    user_id = uuid.uuid4()
    profile_id = uuid.uuid4()

    existing_profile = Profile(
        id=profile_id,
        user_id=user_id,
        profile_type=ProfileType.CUSTOMER,
        first_name="Иван",
    )
    repo_mock.get_by_user_id.return_value = [existing_profile]
    repo_mock.update.return_value = existing_profile

    await service.update_profile(
        profile_id=profile_id,
        user_id=user_id,
        update_data={"first_name": "Пётр"},
    )

    cache_mock.delete.assert_called_once_with(f"profiles:{user_id}")


@pytest.mark.asyncio
async def test_process_like_event_add_success(mock_service):
    service, repo_mock, uow_mock, _, _ = mock_service
    user_id = uuid.uuid4()
    product_id = uuid.uuid4()
    event_data = {
        "user_id": str(user_id),
        "product_id": str(product_id),
        "action": "add",
    }

    await service.process_like_event(event_data)

    repo_mock.add_like.assert_called_once_with(user_id=user_id, product_id=product_id)
    uow_mock.commit.assert_called_once()
    uow_mock.rollback.assert_not_called()


@pytest.mark.asyncio
async def test_process_like_event_remove_success(mock_service):
    service, repo_mock, uow_mock, _, _ = mock_service
    user_id = uuid.uuid4()
    product_id = uuid.uuid4()
    event_data = {
        "user_id": str(user_id),
        "product_id": str(product_id),
        "action": "remove",
    }

    await service.process_like_event(event_data)

    repo_mock.remove_like.assert_called_once_with(
        user_id=user_id, product_id=product_id
    )
    uow_mock.commit.assert_called_once()
    uow_mock.rollback.assert_not_called()


@pytest.mark.asyncio
async def test_process_like_event_db_error_rolls_back(mock_service):
    service, repo_mock, uow_mock, _, _ = mock_service
    repo_mock.add_like.side_effect = RuntimeError("DB connection error")

    event_data = {
        "user_id": str(uuid.uuid4()),
        "product_id": str(uuid.uuid4()),
        "action": "add",
    }

    with pytest.raises(RuntimeError):
        await service.process_like_event(event_data)

    uow_mock.commit.assert_not_called()
    uow_mock.rollback.assert_called_once()


def _products_page(next_cursor: str | None = None) -> BusinessProductsPageDTO:
    now = datetime.now(timezone.utc)
    return BusinessProductsPageDTO(
        products=[
            BusinessProductDTO(
                product_id=str(uuid.uuid4()),
                title="Чайник",
                description=None,
                price=1500,
                image_url="http://minio/image.png",
                rating=4.5,
                feedbacks_count=2,
                created_at=now,
                updated_at=now,
            )
        ],
        next_cursor=next_cursor,
    )


@pytest.mark.asyncio
async def test_get_business_products_delegates_to_catalog(mock_service):
    service, *_ = mock_service
    user_id = uuid.uuid4()
    page = _products_page()
    service.catalog_client.get_business_products.return_value = page

    result = await service.get_business_products(user_id, "cursor", 10)

    assert result is page
    service.catalog_client.get_business_products.assert_awaited_once_with(
        user_id, "cursor", 10
    )


@pytest.mark.asyncio
async def test_get_business_products_api_success(
    mock_profile_service, override_get_service
):
    user_id = str(uuid.uuid4())
    cursor = str(uuid.uuid4())
    next_cursor = str(uuid.uuid4())
    page = _products_page(next_cursor=next_cursor)
    mock_profile_service.get_business_products.return_value = page

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.get(
            "/api/v1/profiles/business/products",
            params={"cursor": cursor, "limit": 10},
            headers={"X-User-Id": user_id, "X-User-Role": "business"},
        )

    assert response.status_code == 200
    data = response.json()
    assert data["next_cursor"] == next_cursor
    assert len(data["products"]) == 1
    product = data["products"][0]
    assert product["product_id"] == page.products[0].product_id
    assert product["title"] == "Чайник"
    assert product["description"] is None
    assert product["price"] == 1500
    assert product["image_url"] == "http://minio/image.png"

    mock_profile_service.get_business_products.assert_awaited_once_with(
        uuid.UUID(user_id), cursor, 10
    )


@pytest.mark.asyncio
async def test_get_business_products_api_defaults(
    mock_profile_service, override_get_service
):
    user_id = str(uuid.uuid4())
    mock_profile_service.get_business_products.return_value = BusinessProductsPageDTO(
        products=[], next_cursor=None
    )

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.get(
            "/api/v1/profiles/business/products",
            headers={"X-User-Id": user_id, "X-User-Role": "business"},
        )

    assert response.status_code == 200
    assert response.json() == {"products": [], "next_cursor": None}
    mock_profile_service.get_business_products.assert_awaited_once_with(
        uuid.UUID(user_id), None, 50
    )


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "invalid_payload",
    [
        {},  # пустой словарь
        {"user_id": "bad-uuid", "product_id": str(uuid.uuid4()), "action": "add"},
        {"user_id": str(uuid.uuid4()), "action": "add"},  # отсутствует product_id
        {
            "user_id": str(uuid.uuid4()),
            "product_id": str(uuid.uuid4()),
            "action": "unknown_action",
        },
    ],
)
async def test_process_like_event_malformed_payload_does_not_call_repo(
    mock_service, invalid_payload
):
    service, repo_mock, uow_mock, _, _ = mock_service

    await service.process_like_event(invalid_payload)

    repo_mock.add_like.assert_not_called()
    repo_mock.remove_like.assert_not_called()
    uow_mock.commit.assert_not_called()


@pytest.mark.asyncio
@pytest.mark.parametrize(
    "params",
    [{"cursor": "not-a-uuid"}, {"limit": 0}, {"limit": 101}],
)
async def test_get_business_products_api_invalid_query(
    mock_profile_service, override_get_service, params
):
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.get(
            "/api/v1/profiles/business/products",
            params=params,
            headers={"X-User-Id": str(uuid.uuid4()), "X-User-Role": "business"},
        )

    assert response.status_code == 422
    mock_profile_service.get_business_products.assert_not_called()


@pytest.mark.asyncio
async def test_get_business_products_api_catalog_unavailable(
    mock_profile_service, override_get_service
):
    mock_profile_service.get_business_products.side_effect = RuntimeError(
        "RPC service error [UNAVAILABLE]: down"
    )

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.get(
            "/api/v1/profiles/business/products",
            headers={"X-User-Id": str(uuid.uuid4()), "X-User-Role": "business"},
        )

    assert response.status_code == 502
    assert response.json()["detail"] == "Catalog service unavailable"


@pytest.mark.asyncio
@pytest.mark.parametrize(
    ("headers", "expected_status"),
    [
        ({"X-User-Role": "customer"}, 403),
        ({"X-User-Role": "admin"}, 403),
        ({"X-User-Role": ""}, 401),
        ({}, 401),
    ],
)
async def test_get_business_products_api_only_business(
    mock_profile_service, override_get_service, headers, expected_status
):
    """Без роли business запрос до сервиса не доходит."""
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.get(
            "/api/v1/profiles/business/products",
            headers={"X-User-Id": str(uuid.uuid4()), **headers},
        )

    assert response.status_code == expected_status
    mock_profile_service.get_business_products.assert_not_called()
