from abc import ABC, abstractmethod


class ICache(ABC):
    """Интерфейс для операций кэширования."""

    @abstractmethod
    async def get(self, key: str) -> str | None:
        """Получить значение по ключу."""

    @abstractmethod
    async def set(self, key: str, value: str, ttl: int | None = None) -> None:
        """Сохранить строковое значение с указанием TTL в секундах."""

    @abstractmethod
    async def delete(self, key: str) -> None:
        """Удалить ключ из кэша."""
