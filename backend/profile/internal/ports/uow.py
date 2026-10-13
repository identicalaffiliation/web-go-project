from abc import ABC, abstractmethod


class IUnitOfWork(ABC):
    """
    Паттерн Unit of Work.
    Обеспечивает атомарность бизнес-операций (все изменения сохраняются вместе или откатываются).
    """

    @abstractmethod
    async def commit(self) -> None:
        pass

    @abstractmethod
    async def rollback(self) -> None:
        pass
