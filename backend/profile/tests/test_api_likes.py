import uuid
from datetime import datetime, timezone

import pytest
from httpx import ASGITransport, AsyncClient

from apps.profile.main import app
from internal.domain.models import WishlistItemDTO


@pytest.mark.asyncio
async def test_get_my_likes_api_success(mock_profile_service, override_get_service):
    user_id = str(uuid.uuid4())
    prod_id = str(uuid.uuid4())
    now = datetime.now(timezone.utc)

    mock_profile_service.get_my_likes.return_value = [
        WishlistItemDTO(product_id=prod_id, added_at=now)
    ]

    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        response = await ac.get(
            "/api/v1/profiles/likes",
            headers={"X-User-Id": user_id},
        )

    assert response.status_code == 200
    data = response.json()
    assert len(data) == 1
    assert data[0]["product_id"] == prod_id
    assert "added_at" in data[0]


@pytest.mark.asyncio
async def test_get_my_likes_api_missing_or_invalid_header(override_get_service):
    async with AsyncClient(
        transport=ASGITransport(app=app), base_url="http://test"
    ) as ac:
        res_missing = await ac.get("/api/v1/profiles/likes")
        assert res_missing.status_code in (400, 422)

        res_invalid = await ac.get(
            "/api/v1/profiles/likes",
            headers={"X-User-Id": "not-a-valid-uuid"},
        )
        assert res_invalid.status_code == 400
