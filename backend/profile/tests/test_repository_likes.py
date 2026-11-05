import uuid

import pytest

from internal.adapters.postgres.repository import SqlAlchemyProfileRepository


@pytest.mark.asyncio
async def test_add_like_idempotency_real_db(test_db_setup):
    maker = test_db_setup
    user_id = uuid.uuid4()
    product_id = uuid.uuid4()

    async with maker() as session:
        repo = SqlAlchemyProfileRepository(session)
        # Первое добавление
        await repo.add_like(user_id, product_id)
        await session.commit()

        await repo.add_like(user_id, product_id)
        await session.commit()

        likes = await repo.get_likes_by_user_id(user_id)
        assert len(likes) == 1
        assert likes[0].product_id == str(product_id)


@pytest.mark.asyncio
async def test_remove_like_non_existent_does_not_fail(test_db_setup):
    maker = test_db_setup
    async with maker() as session:
        repo = SqlAlchemyProfileRepository(session)
        await repo.remove_like(uuid.uuid4(), uuid.uuid4())
        await session.commit()


@pytest.mark.asyncio
async def test_get_likes_filters_by_user_and_orders_by_date(test_db_setup):
    maker = test_db_setup
    user1 = uuid.uuid4()
    user2 = uuid.uuid4()
    prod1 = uuid.uuid4()
    prod2 = uuid.uuid4()

    async with maker() as session:
        repo = SqlAlchemyProfileRepository(session)
        await repo.add_like(user1, prod1)
        await repo.add_like(user1, prod2)
        await repo.add_like(user2, prod1)
        await session.commit()

        likes_user1 = await repo.get_likes_by_user_id(user1)
        likes_user2 = await repo.get_likes_by_user_id(user2)

        assert len(likes_user1) == 2
        assert len(likes_user2) == 1
        # Проверяем, что чужой лайк не попал в выборку
        assert {l.product_id for l in likes_user1} == {str(prod1), str(prod2)}
