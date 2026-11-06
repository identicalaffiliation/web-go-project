import json
import logging
import uuid
from datetime import datetime, timezone
from typing import Any

from sqlalchemy.exc import IntegrityError

from internal.adapters.http.schemas import CreateBusinessProfileRequest
from internal.adapters.kafka.producer import KafkaEventProducer
from internal.config.config import settings
from internal.domain.exceptions import ProfileAlreadyExistsError, ProfileNotFoundError
from internal.domain.models import (
    BalanceDTO,
    BusinessProductsPageDTO,
    OrderDTO,
    Profile,
    ProfileType,
)
from internal.ports.cache import ICache
from internal.ports.repository import IProfileRepository
from internal.ports.rpc_clients import ICatalogClient, IOrdersClient, IPaymentsClient
from internal.ports.uow import IUnitOfWork

logger = logging.getLogger(__name__)


class ProfileService:
    """
    Слой Use Case (Бизнес-логика).
    Оркестрирует доменные модели, репозиторий, кэш и управление транзакциями.
    """

    def __init__(
        self,
        repo: IProfileRepository,
        uow: IUnitOfWork,
        orders_client: IOrdersClient,
        payments_client: IPaymentsClient,
        catalog_client: ICatalogClient,
        event_producer: KafkaEventProducer | None = None,
        cache: ICache | None = None,
    ):
        self.repo = repo
        self.uow = uow
        self.orders_client = orders_client
        self.payments_client = payments_client
        self.catalog_client = catalog_client
        self.event_producer = event_producer
        self.cache = cache

    @staticmethod
    def _profiles_to_json(profiles: list[Profile]) -> str:
        data = [
            {
                "id": str(p.id),
                "user_id": str(p.user_id),
                "profile_type": p.profile_type.value,
                "first_name": p.first_name,
                "last_name": p.last_name,
                "phone": p.phone,
                "company_name": p.company_name,
                "created_at": p.created_at.isoformat() if p.created_at else None,
                "updated_at": p.updated_at.isoformat() if p.updated_at else None,
            }
            for p in profiles
        ]
        return json.dumps(data)

    @staticmethod
    def _json_to_profiles(raw_json: str) -> list[Profile]:
        data = json.loads(raw_json)
        profiles = []
        for item in data:
            profiles.append(
                Profile(
                    id=uuid.UUID(item["id"]),
                    user_id=uuid.UUID(item["user_id"]),
                    profile_type=ProfileType(item["profile_type"]),
                    first_name=item.get("first_name"),
                    last_name=item.get("last_name"),
                    phone=item.get("phone"),
                    company_name=item.get("company_name"),
                    created_at=(
                        datetime.fromisoformat(item["created_at"])
                        if item.get("created_at")
                        else None
                    ),
                    updated_at=(
                        datetime.fromisoformat(item["updated_at"])
                        if item.get("updated_at")
                        else None
                    ),
                )
            )
        return profiles

    async def _invalidate_user_profiles_cache(self, user_id: uuid.UUID) -> None:
        if self.cache:
            await self.cache.delete(f"profiles:{user_id}")

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
                await self._invalidate_user_profiles_cache(user_id)
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
        Получить все профили пользователя с проверкой кэша.
        """
        cache_key = f"profiles:{user_id}"

        if self.cache:
            cached_data = await self.cache.get(cache_key)
            if cached_data:
                try:
                    return self._json_to_profiles(cached_data)
                except Exception:
                    logger.exception("Failed to parse cached profiles for %s", user_id)

        profiles = await self.repo.get_by_user_id(user_id)

        if self.cache and profiles:
            try:
                serialized = self._profiles_to_json(profiles)
                await self.cache.set(
                    cache_key,
                    serialized,
                    ttl=settings.CACHE_TTL_PROFILES_SEC,
                )
            except Exception:
                logger.exception("Failed to cache profiles for %s", user_id)

        return profiles

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
            await self._invalidate_user_profiles_cache(user_id)
            return updated_profile

        except Exception:
            await self.uow.rollback()
            raise

    async def get_orders(self, user_id: uuid.UUID) -> list[OrderDTO]:
        return await self.orders_client.get_user_orders(user_id)

    async def get_balance(self, user_id: uuid.UUID) -> BalanceDTO:
        return await self.payments_client.get_user_balance(user_id)

    async def get_business_products(
        self, user_id: uuid.UUID, cursor: str | None, limit: int
    ) -> BusinessProductsPageDTO:
        """
        Товары, выложенные бизнесом в каталог: владелец товара - user_id бизнеса.
        """
        return await self.catalog_client.get_business_products(user_id, cursor, limit)

    async def create_business_profile(
        self, user_id: uuid.UUID, data: CreateBusinessProfileRequest
    ) -> Profile:
        existing_profile = await self.repo.get_by_user_id_and_type(
            user_id=user_id, profile_type=ProfileType.BUSINESS
        )
        if existing_profile:
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
                profile=new_profile, idempotency_key=uuid.uuid4()
            )
            if not created_profile:
                await self.uow.rollback()
                raise RuntimeError("Failed to create business profile")
            await self.uow.commit()
            await self._invalidate_user_profiles_cache(user_id)
        except IntegrityError:
            await self.uow.rollback()
            raise ProfileAlreadyExistsError(
                "У данного пользователя уже есть бизнес-профиль"
            ) from None
        except Exception as e:
            await self.uow.rollback()
            logger.exception(f"Error creating business profile for user {user_id}")
            raise e

        if self.event_producer:
            event_payload = {
                "event_id": str(uuid.uuid4()),
                "user_id": str(user_id),
                "occurred_at": datetime.now(timezone.utc).isoformat(),
            }
            try:
                await self.event_producer.publish_event(
                    topic=settings.KAFKA_BUSINESS_CREATED_TOPIC,
                    key=str(user_id),
                    payload=event_payload,
                )
            except Exception:
                logger.error(
                    f"Failed to publish business creation event for user {user_id}"
                )

        return created_profile
