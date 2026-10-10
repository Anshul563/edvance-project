from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.moderation_audit_log import ModerationAuditLog


class AuditRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def create(self, payload: dict) -> ModerationAuditLog:
        log = ModerationAuditLog(**payload)
        self.session.add(log)
        await self.session.flush()
        return log

    async def get_for_case(self, case_id: str):
        stmt = select(ModerationAuditLog).where(ModerationAuditLog.case_id == case_id)
        result = await self.session.execute(stmt)
        return result.scalars().all()
