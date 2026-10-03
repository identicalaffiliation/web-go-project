import uuid

from internal.adapters.postgres.models import ProfileModel
from internal.domain.exceptions import ProfileNotFoundError
from internal.domain.models import Profile, ProfileType
from internal.ports.repository import IProfileRepository
from sqlalchemy import select, update
from sqlalchemy.dialects.postgresql import insert
from sqlalchemy.ext.asyncio import AsyncSession


class SqlAlchemyProfileRepository(IProfileRepository):
    def __init__(self, session: AsyncSession):
        self.session = session

    def _to_domain(self, model: ProfileModel) -> Profile:
        """Вспомогательный метод для маппинга ORM модели в чистую Доменную модель"""
        return Profile(
            id=model.id,
            user_id=model.user_id,
            profile_type=model.profile_type,
            first_name=model.first_name,
            last_name=model.last_name,
            phone=model.phone,
            company_name=model.company_name,
            created_at=model.created_at,
            updated_at=model.updated_at,
        )

    async def create(
        self, profile: Profile, idempotency_key: uuid.UUID
    ) -> Profile | None:
        stmt = (
            insert(ProfileModel)
            .values(
                id=profile.id,
                user_id=profile.user_id,
                profile_type=profile.profile_type,
                first_name=profile.first_name,
                last_name=profile.last_name,
                phone=profile.phone,
                company_name=profile.company_name,
                idempotency_key=idempotency_key,
            )
            .on_conflict_do_nothing(index_elements=["idempotency_key"])
            .returning(ProfileModel)
        )

        result = await self.session.execute(stmt)
        model = result.scalar_one_or_none()

        if model is None:
            return None

        return self._to_domain(model)

    async def get_by_user_id(self, user_id: uuid.UUID) -> list[Profile]:
        stmt = select(ProfileModel).where(ProfileModel.user_id == user_id)
        result = await self.session.execute(stmt)
        models = result.scalars().all()
        return [self._to_domain(m) for m in models]

    async def get_by_user_id_and_type(
        self, user_id: uuid.UUID, profile_type: ProfileType
    ) -> Profile | None:
        stmt = select(ProfileModel).where(
            ProfileModel.user_id == user_id, ProfileModel.profile_type == profile_type
        )
        result = await self.session.execute(stmt)
        model = result.scalar_one_or_none()
        return self._to_domain(model) if model else None

    async def update(self, profile: Profile) -> Profile:
        stmt = (
            update(ProfileModel)
            .where(ProfileModel.id == profile.id)
            .values(
                first_name=profile.first_name,
                last_name=profile.last_name,
                phone=profile.phone,
                company_name=profile.company_name,
            )
            .returning(ProfileModel)
        )

        result = await self.session.execute(stmt)
        updated_model = result.scalar_one_or_none()

        if not updated_model:
            raise ProfileNotFoundError(profile_id=str(profile.id))

        return self._to_domain(updated_model)
