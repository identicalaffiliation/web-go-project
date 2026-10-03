import logging
import uuid
from typing import Any

from internal.domain.exceptions import ProfileNotFoundError
from internal.domain.models import Profile, ProfileType
from internal.ports.repository import IProfileRepository
from internal.ports.uow import IUnitOfWork

logger = logging.getLogger(__name__)


class ProfileService:
    """
    Слой Use Case (Бизнес-логика).
    Оркестрирует доменные модели, репозиторий и управление транзакциями.
    """

    def __init__(self, repo: IProfileRepository, uow: IUnitOfWork):

        self.repo = repo
        self.uow = uow

    async def process_registration_event(self, event_data: dict[str, Any]) -> None:
        """
        Обрабатывает событие из Kafka (например, auth.user.registered).
        """
        user_id_str = event_data.get("user_id")
        event_id_str = event_data.get("event_id")  # Это наш idempotency_key

        if not user_id_str or not event_id_str:
            logger.error(f"Invalid event data received from Kafka: {event_data}")
            return

        user_id = uuid.UUID(user_id_str)
        idempotency_key = uuid.UUID(event_id_str)

        new_profile = Profile(
            id=uuid.uuid4(), user_id=user_id, profile_type=ProfileType.CUSTOMER
        )

        try:
            created_profile = await self.repo.create(new_profile, idempotency_key)

            if created_profile:
                await self.uow.commit()
                logger.info(f"Successfully created CUSTOMER profile for user {user_id}")
            else:
                logger.info(
                    f"Event {idempotency_key} already processed (Idempotency). Skipping."
                )

        except Exception:
            await self.uow.rollback()
            logger.exception(f"Failed to create profile for user {user_id}")
            raise

    async def get_profiles_by_user(self, user_id: uuid.UUID) -> list[Profile]:
        """
        Получить все профили пользователя. Транзакция здесь не нужна (только чтение).
        """
        return await self.repo.get_by_user_id(user_id)

    async def update_profile(
        self, profile_id: uuid.UUID, user_id: uuid.UUID, update_data: dict[str, Any]
    ) -> Profile:
        """
        Обновляет данные профиля, предварительно проверяя права доступа.
        """
        # 1. Получаем все профили текущего юзера
        user_profiles = await self.repo.get_by_user_id(user_id)

        # 2. Ищем среди них тот, который запросили на обновление (Проверка принадлежности)
        target_profile = next((p for p in user_profiles if p.id == profile_id), None)

        if not target_profile:
            # Если профиля нет или он чужой — выбрасываем нашу доменную ошибку
            raise ProfileNotFoundError(str(profile_id))

        # 3. Применяем новые данные к бизнес-модели (PATCH-логика)
        if "first_name" in update_data:
            target_profile.first_name = update_data["first_name"]
        if "last_name" in update_data:
            target_profile.last_name = update_data["last_name"]
        if "phone" in update_data:
            target_profile.phone = update_data["phone"]
        if "company_name" in update_data:
            target_profile.company_name = update_data["company_name"]

        try:
            # 4. Передаем обновленную модель в репозиторий и коммитим
            updated_profile = await self.repo.update(target_profile)
            await self.uow.commit()
            return updated_profile

        except Exception:
            await self.uow.rollback()
            raise
