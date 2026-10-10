from __future__ import annotations

from datetime import UTC, datetime

from fastapi import APIRouter, Header, HTTPException, status
from pydantic import BaseModel, Field

from app.config import get_settings
from app.db.session import AsyncSessionLocal
from app.dependencies import require_authentication
from app.repositories.conversation_repository import ConversationRepository
from app.services.quiz_service import QuizService
from app.services.summary_service import SummaryService
from app.services.tutor_service import TutorService
from app.services.usage_service import UsageService

settings = get_settings()
router = APIRouter(prefix="/ai")


class TutorRequest(BaseModel):
    question: str = Field(..., min_length=1, max_length=settings["max_question_length"])
    course_id: str | None = None
    lesson_id: str | None = None
    conversation_id: str | None = None


class SummaryRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=settings["max_summary_chars"])
    source: str | None = None


class QuizRequest(BaseModel):
    source_text: str = Field(..., min_length=1, max_length=settings["max_summary_chars"])
    num_questions: int = Field(default=5, ge=1, le=settings["ai_max_quiz_questions"])
    difficulty: str = Field(default="medium")
    source_reference: str | None = None


class ConversationCreateRequest(BaseModel):
    course_id: str | None = None
    title: str | None = None


async def _record_usage(
    *,
    user_id: str,
    operation: str,
    provider: str,
    model: str,
    status: str,
    latency_ms: int,
    conversation_id: str | None = None,
    request_id: str | None = None,
    input_tokens: int | None = None,
    output_tokens: int | None = None,
):
    async with AsyncSessionLocal() as session:
        service = UsageService(session)
        await service.record_usage(
            user_id=user_id,
            provider=provider,
            model=model,
            operation=operation,
            status=status,
            latency_ms=latency_ms,
            conversation_id=conversation_id,
            request_id=request_id,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )
        await session.commit()


@router.post("/ask")
async def ask_question(
    payload: TutorRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    conversation_id = payload.conversation_id
    async with AsyncSessionLocal() as session:
        repo = ConversationRepository(session)
        conversation = None
        if conversation_id:
            conversation = await repo.get_by_id(conversation_id, user_id=user_id)
            if conversation is None:
                raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
        else:
            title = (payload.question[:80] + "...") if len(payload.question) > 80 else payload.question
            conversation = await repo.create_conversation(
                user_id=user_id,
                course_id=payload.course_id,
                title=title or "AI tutor conversation",
            )
            await session.flush()

        await repo.add_message(
            conversation_id=conversation.id,
            role="user",
            content=payload.question,
        )

        result = await TutorService().ask_question(
            user_id=user_id,
            question=payload.question,
            course_id=payload.course_id,
            lesson_id=payload.lesson_id,
            conversation_id=conversation.id,
            token=authorization,
        )

        await repo.add_message(
            conversation_id=conversation.id,
            role="assistant",
            content=result["answer"],
            source_references={"source_references": result.get("source_references", [])},
        )
        conversation.updated_at = datetime.now(UTC)
        conversation.title = (payload.question[:80] + "...") if len(payload.question) > 80 else payload.question
        await session.commit()

        await _record_usage(
            user_id=user_id,
            operation="tutor.ask",
            provider=result.get("provider") or settings["ai_provider"],
            model=result.get("model") or settings["ai_model"],
            status="success",
            latency_ms=result.get("latency_ms", 0),
            conversation_id=conversation.id,
        )

    return {
        "conversation_id": conversation.id,
        "answer": result["answer"],
        "grounded": result.get("grounded", False),
        "source_references": result.get("source_references", []),
    }


@router.post("/summarize")
async def summarize(payload: SummaryRequest, authorization: str | None = Header(default=None, alias="Authorization")):
    user_id = require_authentication(authorization)
    result = await SummaryService().summarize(text=payload.text, source=payload.source)
    await _record_usage(
        user_id=user_id,
        operation="summary.generate",
        provider=result.get("provider") or settings["ai_provider"],
        model=result.get("model") or settings["ai_model"],
        status="success",
        latency_ms=0,
    )
    return result


@router.post("/quiz")
async def generate_quiz(
    payload: QuizRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    result = await QuizService().generate_quiz(
        source_text=payload.source_text,
        num_questions=payload.num_questions,
        difficulty=payload.difficulty,
        source_reference=payload.source_reference,
    )
    await _record_usage(
        user_id=user_id,
        operation="quiz.generate",
        provider=result.get("provider") or settings["ai_provider"],
        model=result.get("model") or settings["ai_model"],
        status="success",
        latency_ms=0,
    )
    return result


@router.post("/conversations")
async def create_conversation(
    payload: ConversationCreateRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        repo = ConversationRepository(session)
        conversation = await repo.create_conversation(
            user_id=user_id,
            course_id=payload.course_id,
            title=payload.title or "New AI conversation",
        )
        await session.commit()
    return {"conversation_id": conversation.id, "title": conversation.title}


@router.get("/conversations")
async def list_conversations(
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        repo = ConversationRepository(session)
        conversations = await repo.list_for_user(user_id)
    return {
        "items": [
            {
                "id": conversation.id,
                "title": conversation.title,
                "course_id": conversation.course_id,
                "created_at": conversation.created_at.isoformat(),
                "updated_at": conversation.updated_at.isoformat(),
            }
            for conversation in conversations
        ]
    }


@router.get("/conversations/{conversation_id}")
async def get_conversation(
    conversation_id: str,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        repo = ConversationRepository(session)
        conversation = await repo.get_by_id(conversation_id, user_id=user_id)
        if conversation is None:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
    return {
        "id": conversation.id,
        "title": conversation.title,
        "course_id": conversation.course_id,
        "created_at": conversation.created_at.isoformat(),
        "updated_at": conversation.updated_at.isoformat(),
    }


@router.get("/conversations/{conversation_id}/messages")
async def list_messages(
    conversation_id: str,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        repo = ConversationRepository(session)
        conversation = await repo.get_by_id(conversation_id, user_id=user_id)
        if conversation is None:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="conversation not found")
        messages = await repo.list_messages(conversation_id)
    return {
        "conversation_id": conversation_id,
        "items": [
            {
                "id": message.id,
                "role": message.role,
                "content": message.content,
                "source_references": message.source_references,
                "created_at": message.created_at.isoformat(),
            }
            for message in messages
        ],
    }
