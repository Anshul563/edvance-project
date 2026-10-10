from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.moderation_action import ModerationAction


class ActionRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def get_by_id(self, action_id: str) -> ModerationAction | None:
        result = await self.session.execute(
            select(ModerationAction).where(ModerationAction.id == action_id)
        )
        return result.scalar_one_or_none()

    async def get_for_case(self, case_id: str):
        stmt = select(ModerationAction).where(ModerationAction.case_id == case_id)
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def create(self, payload: dict) -> ModerationAction:
        action = ModerationAction(**payload)
        self.session.add(action)
        await self.session.flush()
        return action
