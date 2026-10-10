from __future__ import annotations

from datetime import datetime, timedelta, timezone
from uuid import UUID

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.db.models import (
    AnalyticsCourseDaily,
    AnalyticsCreatorDaily,
    AnalyticsEvent,
    AnalyticsPlatformDaily,
)
from app.repositories.event_repository import EventRepository


class AnalyticsProcessor:
    def __init__(self, session: AsyncSession):
        self.session = session
        self.repo = EventRepository(session)

    async def process_batch(self, limit: int = 100) -> int:
        events = await self.repo.claim_pending_batch(limit)
        for event in events:
            try:
                await self._process_event(event)
                await self.repo.mark_processed(event)
            except Exception as exc:  # pragma: no cover - defensive path
                await self.repo.mark_failed(event, str(exc))
        return len(events)

    async def _process_event(self, event: AnalyticsEvent) -> None:
        if event.event_type == "content_view":
            await self._apply_content_view(event)
        elif event.event_type == "course_enrollment":
            await self._apply_course_enrollment(event)
        elif event.event_type == "course_completion":
            await self._apply_course_completion(event)
        elif event.event_type == "creator_follow":
            await self._apply_creator_follow(event)
        elif event.event_type == "payment_received":
            await self._apply_payment_received(event)
        else:
            raise ValueError(f"unsupported event type: {event.event_type}")

    async def _apply_content_view(self, event: AnalyticsEvent) -> None:
        current_date = event.occurred_at.date()
        creator_id = event.properties.get("creator_id") or event.entity_id
        row = await self._get_or_create_creator_daily(creator_id, current_date)
        row.content_views += 1
        row.watch_time_seconds += int(event.properties.get("watch_time_seconds", 0) or 0)
        row.updated_at = datetime.now(timezone.utc)

    async def _apply_course_enrollment(self, event: AnalyticsEvent) -> None:
        current_date = event.occurred_at.date()
        course_id = event.entity_id
        row = await self._get_or_create_course_daily(course_id, current_date)
        row.enrollments += 1
        row.updated_at = datetime.now(timezone.utc)

    async def _apply_course_completion(self, event: AnalyticsEvent) -> None:
        current_date = event.occurred_at.date()
        course_id = event.entity_id
        row = await self._get_or_create_course_daily(course_id, current_date)
        row.completions += 1
        row.completion_rate = (row.completions / max(row.enrollments, 1)) * 100
        row.updated_at = datetime.now(timezone.utc)

    async def _apply_creator_follow(self, event: AnalyticsEvent) -> None:
        current_date = event.occurred_at.date()
        creator_id = event.entity_id
        row = await self._get_or_create_creator_daily(creator_id, current_date)
        row.followers_gained += 1
        row.updated_at = datetime.now(timezone.utc)

    async def _apply_payment_received(self, event: AnalyticsEvent) -> None:
        amount_cents = int(event.properties.get("amount_cents", 0) or 0)
        current_date = event.occurred_at.date()
        creator_id = event.properties.get("creator_id") or event.entity_id
        row = await self._get_or_create_creator_daily(creator_id, current_date)
        row.revenue_cents += amount_cents

        platform_row = await self._get_or_create_platform_daily(current_date)
        platform_row.gross_revenue_cents += amount_cents
        platform_row.net_revenue_cents = platform_row.gross_revenue_cents - platform_row.refunds_cents
        platform_row.updated_at = datetime.now(timezone.utc)
        row.updated_at = datetime.now(timezone.utc)

    async def _get_or_create_creator_daily(self, creator_id: str, target_date: datetime.date) -> AnalyticsCreatorDaily:
        result = await self.session.execute(
            select(AnalyticsCreatorDaily).where(
                AnalyticsCreatorDaily.creator_id == creator_id,
                AnalyticsCreatorDaily.reporting_date == target_date,
            )
        )
        row = result.scalar_one_or_none()
        if row is not None:
            return row
        row = AnalyticsCreatorDaily(
            creator_id=creator_id,
            reporting_date=target_date,
            created_at=datetime.now(timezone.utc),
            updated_at=datetime.now(timezone.utc),
        )
        self.session.add(row)
        return row

    async def _get_or_create_course_daily(self, course_id: str, target_date: datetime.date) -> AnalyticsCourseDaily:
        result = await self.session.execute(
            select(AnalyticsCourseDaily).where(
                AnalyticsCourseDaily.course_id == course_id,
                AnalyticsCourseDaily.reporting_date == target_date,
            )
        )
        row = result.scalar_one_or_none()
        if row is not None:
            return row
        row = AnalyticsCourseDaily(
            course_id=course_id,
            reporting_date=target_date,
            created_at=datetime.now(timezone.utc),
            updated_at=datetime.now(timezone.utc),
        )
        self.session.add(row)
        return row

    async def _get_or_create_platform_daily(self, target_date: datetime.date) -> AnalyticsPlatformDaily:
        result = await self.session.execute(
            select(AnalyticsPlatformDaily).where(AnalyticsPlatformDaily.reporting_date == target_date)
        )
        row = result.scalar_one_or_none()
        if row is not None:
            return row
        row = AnalyticsPlatformDaily(
            reporting_date=target_date,
            created_at=datetime.now(timezone.utc),
            updated_at=datetime.now(timezone.utc),
        )
        self.session.add(row)
        return row
