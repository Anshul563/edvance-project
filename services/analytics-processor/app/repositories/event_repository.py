from __future__ import annotations

from datetime import datetime, timezone

from sqlalchemy import select, update
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import AnalyticsEvent


class EventRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def claim_pending_batch(self, limit: int) -> list[AnalyticsEvent]:
        query = (
            select(AnalyticsEvent)
            .where(AnalyticsEvent.processing_status == "pending")
            .order_by(AnalyticsEvent.occurred_at.asc())
            .limit(limit)
        )
        result = await self.session.execute(query)
        return list(result.scalars().all())

    async def mark_processed(self, event: AnalyticsEvent) -> None:
        event.processing_status = "processed"
        event.processed_at = datetime.now(timezone.utc)
        await self.session.flush()

    async def mark_failed(self, event: AnalyticsEvent, detail: str) -> None:
        event.processing_status = "failed"
        event.properties = {**(event.properties or {}), "processing_error": detail[:512]}
        await self.session.flush()

    async def save_aggregate(self, model, data: dict, key_fields: tuple[str, ...]) -> None:
        row = await self.session.execute(select(model).where(*[getattr(model, field) == data[field] for field in key_fields]))
        existing = row.scalar_one_or_none()
        if existing is None:
            self.session.add(model(**data))
            return
        for field, value in data.items():
            if field in key_fields:
                continue
            setattr(existing, field, value)

    async def update_checkpoint(self, checkpoint_name: str, last_event_id: str) -> None:
        from app.db.models import AnalyticsProcessingCheckpoint
        from uuid import UUID

        checkpoint = await self.session.get(AnalyticsProcessingCheckpoint, UUID(checkpoint_name))
        if checkpoint is None:
            checkpoint = AnalyticsProcessingCheckpoint(
                id=UUID(checkpoint_name),
                checkpoint_name=checkpoint_name,
                last_event_id=UUID(last_event_id),
                last_processed_at=datetime.now(timezone.utc),
            )
            self.session.add(checkpoint)
            return
        checkpoint.last_event_id = UUID(last_event_id)
        checkpoint.last_processed_at = datetime.now(timezone.utc)
