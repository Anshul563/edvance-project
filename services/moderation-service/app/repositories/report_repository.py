from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.content_report import ContentReport


class ReportRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def create(self, payload: dict) -> ContentReport:
        report = ContentReport(**payload)
        self.session.add(report)
        await self.session.flush()
        return report

    async def get_by_id(self, report_id: str) -> ContentReport | None:
        result = await self.session.execute(
            select(ContentReport).where(ContentReport.id == report_id)
        )
        return result.scalar_one_or_none()

    async def list_for_user(self, user_id: str, offset: int = 0, limit: int = 20):
        stmt = (
            select(ContentReport)
            .where(ContentReport.reporter_user_id == user_id)
            .order_by(ContentReport.created_at.desc())
            .offset(offset)
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def exists_recent_duplicate(
        self,
        reporter_user_id: str,
        target_type: str,
        target_id: str,
        reason: str,
    ) -> bool:
        stmt = select(ContentReport.id).where(
            ContentReport.reporter_user_id == reporter_user_id,
            ContentReport.target_type == target_type,
            ContentReport.target_id == target_id,
            ContentReport.reason == reason,
            ContentReport.status.in_({"open", "reviewing", "escalated"}),
        )
        result = await self.session.execute(stmt)
        return result.scalar_one_or_none() is not None
