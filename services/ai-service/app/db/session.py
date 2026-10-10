from __future__ import annotations

from sqlalchemy import StaticPool
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.config import get_settings
from app.db.base import Base
from app.models import AIConversation, AIMessage, AIUsageRecord  # noqa: F401

settings = get_settings()

engine_kwargs = {}
connect_args = {}
if settings["database_url"].startswith("sqlite"):
    engine_kwargs["poolclass"] = StaticPool
    connect_args["check_same_thread"] = False

engine = create_async_engine(settings["database_url"], connect_args=connect_args, **engine_kwargs)
AsyncSessionLocal = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)


async def initialize_database() -> None:
    async with engine.begin() as connection:
        await connection.run_sync(Base.metadata.create_all)


async def get_db_session() -> AsyncSession:
    async with AsyncSessionLocal() as session:
        yield session
