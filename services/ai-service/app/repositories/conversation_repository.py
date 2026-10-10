from __future__ import annotations

from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.models.ai_conversation import AIConversation
from app.models.ai_message import AIMessage


class ConversationRepository:
    def __init__(self, session: AsyncSession):
        self.session = session

    async def create_conversation(self, *, user_id: str, course_id: str | None, title: str) -> AIConversation:
        conversation = AIConversation(user_id=user_id, course_id=course_id, title=title)
        self.session.add(conversation)
        await self.session.flush()
        return conversation

    async def list_for_user(self, user_id: str, offset: int = 0, limit: int = 20):
        stmt = (
            select(AIConversation)
            .where(AIConversation.user_id == user_id)
            .order_by(AIConversation.updated_at.desc())
            .offset(offset)
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def get_by_id(self, conversation_id: str, user_id: str | None = None) -> AIConversation | None:
        stmt = select(AIConversation).where(AIConversation.id == conversation_id)
        if user_id is not None:
            stmt = stmt.where(AIConversation.user_id == user_id)
        result = await self.session.execute(stmt)
        return result.scalar_one_or_none()

    async def add_message(
        self,
        *,
        conversation_id: str,
        role: str,
        content: str,
        source_references: dict | None = None,
    ) -> AIMessage:
        message = AIMessage(
            conversation_id=conversation_id,
            role=role,
            content=content,
            source_references=source_references or {},
        )
        self.session.add(message)
        await self.session.flush()
        return message

    async def list_messages(self, conversation_id: str, limit: int = 50):
        stmt = (
            select(AIMessage)
            .where(AIMessage.conversation_id == conversation_id)
            .order_by(AIMessage.created_at.asc())
            .limit(limit)
        )
        result = await self.session.execute(stmt)
        return result.scalars().all()

    async def delete_conversation(self, conversation_id: str) -> None:
        records = await self.session.execute(select(AIMessage).where(AIMessage.conversation_id == conversation_id))
        for row in records.scalars().all():
            await self.session.delete(row)
        conversation = await self.get_by_id(conversation_id)
        if conversation is not None:
            await self.session.delete(conversation)
        await self.session.flush()
