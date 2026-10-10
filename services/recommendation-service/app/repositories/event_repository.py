from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.recommendation_event import RecommendationEvent


class EventRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def add_event(self, payload: dict):
        event = RecommendationEvent(**payload)
        self.session.add(event)
        await self.session.flush()
        return event

    async def get_recent_for_user(self, user_id: str, limit: int = 50):
        stmt = (
            select(RecommendationEvent)
            .where(RecommendationEvent.user_id == user_id)
            .order_by(RecommendationEvent.created_at.desc())
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        return result.scalars().all()
