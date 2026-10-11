from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    POSTGRES_USER: str
    POSTGRES_PASSWORD: str
    POSTGRES_HOST: str
    POSTGRES_PORT: int
    POSTGRES_DB: str

    KAFKA_BOOTSTRAP_SERVERS: str
    KAFKA_CONSUMER_GROUP: str = "profile_service_group"
    KAFKA_USER_REGISTERED_TOPIC: str = "auth.user.registered"

    ORDERS_GRPC_HOST: str = "localhost:50051"
    PAYMENTS_GRPC_HOST: str = "localhost:50052"

    @property
    def database_url(self) -> str:
        return (
            f"postgresql+asyncpg://{self.POSTGRES_USER}:{self.POSTGRES_PASSWORD}"
            f"@{self.POSTGRES_HOST}:{self.POSTGRES_PORT}/{self.POSTGRES_DB}"
        )

    model_config = SettingsConfigDict(
        env_file="configs/.env",
        env_file_encoding="utf-8",
        extra="ignore",
    )


settings = Settings()
