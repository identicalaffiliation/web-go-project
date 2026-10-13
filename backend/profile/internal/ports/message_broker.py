from abc import ABC, abstractmethod


class IEventConsumer(ABC):
    """
    Интерфейс (порт) консьюмера событий.
    Абстрагирует работу с брокером сообщений (Kafka).
    """

    @abstractmethod
    async def start(self) -> None:
        """
        Устанавливает соединение с брокером и запускает бесконечный цикл
        чтения сообщений в фоновом режиме.
        """

    @abstractmethod
    async def stop(self) -> None:
        """
        Корректно завершает работу консьюмера, отписывается от топиков
        и закрывает сетевые соединения.
        """
