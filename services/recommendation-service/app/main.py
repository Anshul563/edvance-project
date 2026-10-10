from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.api import events, feedback, health, interests, internal, recommendations
from app.db.base import Base
from app.db.session import engine


@asynccontextmanager
async def lifespan(app: FastAPI):
    async with engine.begin() as conn:
        await conn.run_sync(Base.metadata.create_all)
    yield


app = FastAPI(title="Edvance Recommendation Service", version="0.1.0", lifespan=lifespan)

app.include_router(health.router)
app.include_router(recommendations.router, prefix="/api/v1/recommendations")
app.include_router(interests.router, prefix="/api/v1/recommendations")
app.include_router(events.router, prefix="/api/v1/recommendations")
app.include_router(feedback.router, prefix="/api/v1/recommendations")
app.include_router(internal.router, prefix="/internal/v1/recommendations")
