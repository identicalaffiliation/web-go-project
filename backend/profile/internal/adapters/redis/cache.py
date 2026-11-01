import logging

import redis.asyncio as redis

from internal.ports.cache import ICache

logger = logging.getLogger(__name__)


class RedisCache(ICache):
    def __init__(self, redis_client: redis.Redis):
        self._client = redis_client

    async def get(self, key: str) -> str | None:
        try:
            return await self._client.get(key)
        except redis.RedisError:
            logger.exception("Redis error occurred during GET for key: %s", key)
            return None

    async def set(self, key: str, value: str, ttl: int | None = None) -> None:
        try:
            await self._client.set(name=key, value=value, ex=ttl)
        except redis.RedisError:
            logger.exception("Redis error occurred during SET for key: %s", key)

    async def delete(self, key: str) -> None:
        try:
            await self._client.delete(key)
        except redis.RedisError:
            logger.exception("Redis error occurred during DELETE for key: %s", key)
