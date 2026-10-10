from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.ai_usage_record import AIUsageRecord


class UsageRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def create_record(
        self,
        *,
        user_id: str,
        provider: str,
        model: str,
        operation: str,
        status: str,
        latency_ms: int,
        conversation_id: str | None = None,
        request_id: str | None = None,
        input_tokens: int | None = None,
        output_tokens: int | None = None,
    ) -> AIUsageRecord:
        record = AIUsageRecord(
            user_id=user_id,
            conversation_id=conversation_id,
            request_id=request_id,
            provider=provider,
            model=model,
            operation=operation,
            status=status,
            latency_ms=latency_ms,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )
        self.session.add(record)
        await self.session.flush()
        return record

    async def list_for_user(self, user_id: str, offset: int = 0, limit: int = 20):
        stmt = (
            select(AIUsageRecord)
            .where(AIUsageRecord.user_id == user_id)
            .order_by(AIUsageRecord.created_at.desc())
            .offset(offset)
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        return result.scalars().all()
