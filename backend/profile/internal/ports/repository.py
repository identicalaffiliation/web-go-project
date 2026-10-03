import uuid
from abc import ABC, abstractmethod

from internal.domain.models import Profile, ProfileType


class IProfileRepository(ABC):
    """
    Интерфейс (порт) для работы с хранилищем профилей.
    Любой класс, который будет работать с БД, обязан реализовать эти методы.
    """

    @abstractmethod
    async def create(
        self, profile: Profile, idempotency_key: uuid.UUID
    ) -> Profile | None:
        """
        Создает новый профиль в БД.

        :param profile: Доменная модель профиля для сохранения.
        :param idempotency_key: ID сообщения из Kafka для защиты от дублей.
        :return: Созданный профиль или None, если такой idempotency_key уже был (дубль).
        """

    @abstractmethod
    async def get_by_user_id(self, user_id: uuid.UUID) -> list[Profile]:
        """
        Возвращает все профили конкретного пользователя.
        У одного пользователя может быть список из 1-2 профилей (CUSTOMER, BUSINESS).
        """

    @abstractmethod
    async def get_by_user_id_and_type(
        self, user_id: uuid.UUID, profile_type: ProfileType
    ) -> Profile | None:
        """
        Возвращает конкретный профиль пользователя по его типу.
        """

    @abstractmethod
    async def update(self, profile: Profile) -> Profile:
        """
        Обновляет данные существующего профиля.
        """
