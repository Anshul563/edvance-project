from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.content_report import ContentReport
from app.models.moderation_action import ModerationAction
from app.models.moderation_assessment import ModerationAssessment
from app.models.moderation_case import ModerationCase


class CaseRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def get_by_id(self, case_id: str) -> ModerationCase | None:
        result = await self.session.execute(
            select(ModerationCase).where(ModerationCase.id == case_id)
        )
        return result.scalar_one_or_none()

    async def list_cases(
        self,
        status: str | None = None,
        priority: str | None = None,
        target_type: str | None = None,
        assigned_moderator_id: str | None = None,
        offset: int = 0,
        limit: int = 20,
    ):
        stmt = select(ModerationCase)
        if status:
            stmt = stmt.where(ModerationCase.status == status)
        if priority:
            stmt = stmt.where(ModerationCase.priority == priority)
        if target_type:
            stmt = stmt.where(ModerationCase.target_type == target_type)
        if assigned_moderator_id:
            stmt = stmt.where(ModerationCase.assigned_moderator_id == assigned_moderator_id)
        stmt = stmt.order_by(ModerationCase.created_at.desc()).offset(offset).limit(limit)
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def case_reports(self, case_id: str):
        result = await self.session.execute(
            select(ContentReport).where(ContentReport.target_id == case_id)
        )
        return result.scalars().all()

    async def create_for_target(
        self, target_type: str, target_id: str, policy_version: str
    ) -> ModerationCase:
        case = ModerationCase(
            target_type=target_type,
            target_id=target_id,
            status="open",
            priority="normal",
            policy_version=policy_version,
        )
        self.session.add(case)
        await self.session.flush()
        return case

    async def add_assessment(self, data: dict) -> ModerationAssessment:
        assessment = ModerationAssessment(**data)
        self.session.add(assessment)
        await self.session.flush()
        return assessment

    async def add_action(self, data: dict) -> ModerationAction:
        action = ModerationAction(**data)
        self.session.add(action)
        await self.session.flush()
        return action
