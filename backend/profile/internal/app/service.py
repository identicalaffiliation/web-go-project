import logging
import uuid
from typing import Any

from internal.adapters.http.schemas import CreateBusinessProfileRequest
from internal.domain.exceptions import ProfileAlreadyExistsError, ProfileNotFoundError
from internal.domain.models import BalanceDTO, OrderDTO, Profile, ProfileType
from internal.ports.repository import IProfileRepository
from internal.ports.rpc_clients import IOrdersClient, IPaymentsClient
from internal.ports.uow import IUnitOfWork

logger = logging.getLogger(__name__)


class ProfileService:
    """
    Слой Use Case (Бизнес-логика).
    Оркестрирует доменные модели, репозиторий и управление транзакциями.
    """

    def __init__(
        self,
        repo: IProfileRepository,
        uow: IUnitOfWork,
        orders_client: IOrdersClient,
        payments_client: IPaymentsClient,
    ):
        self.repo = repo
        self.uow = uow
        self.orders_client = orders_client
        self.payments_client = payments_client

    async def process_registration_event(self, event_data: dict[str, Any]) -> None:
        """
        Обрабатывает событие из Kafka (например, auth.user.registered).
        """
        user_id_str = event_data.get("user_id")
        event_id_str = event_data.get("event_id")

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
                await self.uow.rollback()
                logger.info(
                    f"Event {idempotency_key} already processed (Idempotency). Skipping."
                )

        except Exception:
            await self.uow.rollback()
            logger.exception(f"Failed to create profile for user {user_id}")
            raise

    async def get_profiles_by_user(self, user_id: uuid.UUID) -> list[Profile]:
        """
        Получить все профили пользователя.
        """
        return await self.repo.get_by_user_id(user_id)

    async def update_profile(
        self, profile_id: uuid.UUID, user_id: uuid.UUID, update_data: dict[str, Any]
    ) -> Profile:
        """
        Обновляет данные профиля, предварительно проверяя права доступа.
        """
        user_profiles = await self.repo.get_by_user_id(user_id)

        target_profile = next((p for p in user_profiles if p.id == profile_id), None)

        if not target_profile:
            raise ProfileNotFoundError(str(profile_id))

        if "first_name" in update_data:
            target_profile.first_name = update_data["first_name"]
        if "last_name" in update_data:
            target_profile.last_name = update_data["last_name"]
        if "phone" in update_data:
            target_profile.phone = update_data["phone"]
        if "company_name" in update_data:
            target_profile.company_name = update_data["company_name"]

        try:
            updated_profile = await self.repo.update(target_profile)
            await self.uow.commit()
            return updated_profile

        except Exception:
            await self.uow.rollback()
            raise

    async def get_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        """Получить историю заказов пользователя из внешнего сервиса."""
        return await self.orders_client.get_user_orders(user_id)

    async def get_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        """Получить баланс пользователя из внешнего сервиса."""
        return await self.payments_client.get_user_balance(user_id)

    async def create_business_profile(
        self, user_id: uuid.UUID, data: CreateBusinessProfileRequest
    ) -> Profile:
        """
        Создает дополнительный профиль типа BUSINESS для существующего пользователя.
        """

        existing_profile = await self.repo.get_by_user_id_and_type(
            user_id=user_id, profile_type=ProfileType.BUSINESS
        )
        if existing_profile:
            logger.warning(f"Business profile already exists for user {user_id}")
            raise ProfileAlreadyExistsError(
                "У данного пользователя уже есть бизнес-профиль"
            )

        new_profile = Profile(
            id=uuid.uuid4(),
            user_id=user_id,
            profile_type=ProfileType.BUSINESS,
            first_name=None,
            last_name=None,
            company_name=data.company_name,
            phone=data.phone,
        )

        try:
            created_profile = await self.repo.create(
                new_profile, idempotency_key=uuid.uuid4()
            )

            if created_profile:
                await self.uow.commit()
                logger.info(f"Successfully created BUSINESS profile for user {user_id}")
                return created_profile
            else:
                await self.uow.rollback()
                raise RuntimeError(
                    "Failed to create business profile (repository returned None)"
                )

        except Exception as e:
            await self.uow.rollback()
            logger.exception(f"Error creating business profile for user {user_id}")
            raise e
