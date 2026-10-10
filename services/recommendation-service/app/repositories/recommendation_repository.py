from __future__ import annotations

from datetime import UTC, datetime

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.recommendation_item import RecommendationItem


class RecommendationRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def upsert_item(self, payload: dict) -> RecommendationItem:
        source_type = payload.get("source_type")
        source_id = payload.get("source_id")
        if source_type is None or source_id is None:
            raise ValueError("source_type and source_id are required")
        stmt = select(RecommendationItem).where(
            RecommendationItem.source_type == source_type,
            RecommendationItem.source_id == source_id,
        )
        result = await self.session.execute(stmt)
        item = result.scalars().first()
        if item is None:
            item = RecommendationItem(
                id=payload.get("id") or str(datetime.now(UTC).timestamp()),
                source_type=source_type,
                source_id=source_id,
            )
            self.session.add(item)
        for key, value in payload.items():
            if key == "metadata":
                item.item_metadata = value
                continue
            if hasattr(item, key):
                setattr(item, key, value)
        item.updated_at = datetime.now(UTC)
        await self.session.flush()
        return item

    async def get_by_type(self, source_type: str | None = None, limit: int = 20, offset: int = 0):
        stmt = (
            select(RecommendationItem)
            .where(RecommendationItem.is_eligible.is_(True))
            .where(RecommendationItem.visibility == "public")
        )
        if source_type and source_type != "all":
            stmt = stmt.where(RecommendationItem.source_type == source_type)
        stmt = stmt.order_by(
            RecommendationItem.engagement_score.desc(),
            RecommendationItem.published_at.desc().nullslast(),
        )
        stmt = stmt.limit(limit).offset(offset)
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def get_similar(self, source_type: str, source_id: str, limit: int = 10):
        stmt = (
            select(RecommendationItem)
            .where(RecommendationItem.is_eligible.is_(True))
            .where(RecommendationItem.visibility == "public")
        )
        stmt = stmt.where(RecommendationItem.source_type == source_type)
        stmt = stmt.where(RecommendationItem.source_id != source_id)
        result = await self.session.execute(stmt)
        items = result.scalars().all()
        if not items:
            return []
        base = next((item for item in items if item.source_id == source_id), None)
        if base is None:
            base = items[0]
        if base.category:
            items = [item for item in items if item.category == base.category]
        return items[:limit]

    async def get_trending(self, limit: int = 20):
        stmt = (
            select(RecommendationItem)
            .where(RecommendationItem.is_eligible.is_(True))
            .where(RecommendationItem.visibility == "public")
        )
        stmt = stmt.order_by(
            RecommendationItem.engagement_score.desc(),
            RecommendationItem.published_at.desc().nullslast(),
        )
        stmt = stmt.limit(limit)
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def delete_item(self, source_type: str, source_id: str) -> bool:
        stmt = (
            select(RecommendationItem)
            .where(RecommendationItem.source_type == source_type)
            .where(RecommendationItem.source_id == source_id)
        )
        result = await self.session.execute(stmt)
        item = result.scalars().first()
        if item is None:
            return False
        await self.session.delete(item)
        await self.session.flush()
        return True
