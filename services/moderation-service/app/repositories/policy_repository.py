from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.moderation_policy import ModerationPolicy


class PolicyRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def list_enabled(self):
        result = await self.session.execute(
            select(ModerationPolicy).where(ModerationPolicy.enabled.is_(True))
        )
        return result.scalars().all()

    async def get_by_id(self, policy_id: str) -> ModerationPolicy | None:
        result = await self.session.execute(
            select(ModerationPolicy).where(ModerationPolicy.id == policy_id)
        )
        return result.scalar_one_or_none()

    async def create(self, policy: ModerationPolicy) -> ModerationPolicy:
        self.session.add(policy)
        await self.session.flush()
        return policy

    async def update(self, policy: ModerationPolicy, payload: dict) -> ModerationPolicy:
        for key, value in payload.items():
            if hasattr(policy, key):
                setattr(policy, key, value)
        await self.session.flush()
        return policy
