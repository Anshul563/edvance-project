from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.user_interest import UserInterest


class InterestRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def list_for_user(self, user_id: str):
        stmt = select(UserInterest).where(UserInterest.user_id == user_id)
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def profile_for_user(self, user_id: str) -> dict[str, list[str]]:
        interests = await self.list_for_user(user_id)
        categories = sorted({interest.category for interest in interests if interest.category})
        tags = sorted({interest.tag for interest in interests if interest.tag})
        return {"categories": categories, "tags": tags}

    async def upsert_for_user(
        self, user_id: str, category: str | None = None, tag: str | None = None, weight: float = 0.0
    ):
        if category is None and tag is None:
            return None
        stmt = select(UserInterest).where(UserInterest.user_id == user_id)
        if category:
            stmt = stmt.where(UserInterest.category == category)
        if tag:
            stmt = stmt.where(UserInterest.tag == tag)
        result = await self.session.execute(stmt)
        interest = result.scalars().first()
        if interest is None:
            interest = UserInterest(user_id=user_id, category=category, tag=tag, weight=weight)
            self.session.add(interest)
        else:
            interest.weight = max(0.0, min(1.0, weight if weight else interest.weight))
        await self.session.flush()
        return interest

    async def set_preferences(
        self, user_id: str, categories: list[str], tags: list[str]
    ) -> list[UserInterest]:
        existing = await self.list_for_user(user_id)
        for item in existing:
            await self.session.delete(item)
        records: list[UserInterest] = []
        for category in categories:
            interest = UserInterest(
                user_id=user_id, category=category.strip(), tag=None, weight=1.0
            )
            records.append(interest)
            self.session.add(interest)
        for tag in tags:
            interest = UserInterest(
                user_id=user_id, category=None, tag=tag.strip().lower(), weight=1.0
            )
            records.append(interest)
            self.session.add(interest)
        await self.session.flush()
        return records
