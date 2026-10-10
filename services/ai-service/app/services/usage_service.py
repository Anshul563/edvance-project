from __future__ import annotations

from uuid import uuid4

from app.repositories.usage_repository import UsageRepository


class UsageService:
    def __init__(self, session):
        self.session = session
        self.repository = UsageRepository(session)

    async def record_usage(
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
    ):
        return await self.repository.create_record(
            user_id=user_id,
            conversation_id=conversation_id,
            request_id=request_id or str(uuid4()),
            provider=provider,
            model=model,
            operation=operation,
            status=status,
            latency_ms=latency_ms,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )

    async def list_for_user(self, user_id: str, *, offset: int = 0, limit: int = 20):
        return await self.repository.list_for_user(user_id, offset=offset, limit=limit)
