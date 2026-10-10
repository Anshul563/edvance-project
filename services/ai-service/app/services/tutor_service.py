from __future__ import annotations

import time
from uuid import uuid4

from app.config import get_settings
from app.providers.configured_provider import ConfiguredProvider
from app.services.context_service import ContextService
from app.services.prompt_service import PromptService
from app.services.retrieval_service import RetrievalService

settings = get_settings()


class TutorService:
    def __init__(self):
        self.provider = ConfiguredProvider()
        self.retrieval = RetrievalService()

    async def ask_question(
        self,
        *,
        user_id: str,
        question: str,
        course_id: str | None = None,
        lesson_id: str | None = None,
        conversation_id: str | None = None,
        token: str | None = None,
    ) -> dict:
        if not question or not question.strip():
            raise ValueError("question is required")
        materials = await self.retrieval.fetch_course_material(course_id=course_id, lesson_id=lesson_id, token=token)
        texts = [entry.get("text", "") for entry in materials.get("material", []) if entry.get("text")]
        context = ContextService.select_context(texts, question)
        if not context:
            return {
                "answer": "The answer cannot be established from the supplied course materials.",
                "grounded": False,
                "source_references": [],
                "conversation_id": conversation_id,
            }
        messages = [
            {"role": "system", "content": PromptService.tutor_system_prompt()},
            {"role": "user", "content": f"Question: {question}\n\nCourse context:\n{context}"},
        ]
        started = time.perf_counter()
        result = await self.provider.generate_response(messages=messages, context=context, response_format="json")
        elapsed = int((time.perf_counter() - started) * 1000)
        payload = result.get("content") if isinstance(result.get("content"), dict) else {"answer": result.get("content", "")}
        answer = payload.get("answer") or payload.get("content") or "No answer available."
        sources = payload.get("source_references") or []
        grounded = bool(payload.get("grounded", True) and bool(context and sources or ("cannot be established" not in str(answer).lower())))
        return {
            "answer": answer,
            "grounded": grounded,
            "source_references": sources,
            "conversation_id": conversation_id or str(uuid4()),
            "provider": result.get("metadata", {}).get("provider"),
            "model": result.get("metadata", {}).get("model"),
            "latency_ms": elapsed,
        }
