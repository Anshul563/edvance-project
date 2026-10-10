from __future__ import annotations

from app.config import get_settings
from app.providers.configured_provider import ConfiguredProvider
from app.services.context_service import ContextService
from app.services.prompt_service import PromptService

settings = get_settings()


class QuizService:
    def __init__(self):
        self.provider = ConfiguredProvider()

    async def generate_quiz(
        self,
        *,
        source_text: str,
        num_questions: int = 5,
        difficulty: str = "medium",
        source_reference: str | None = None,
    ) -> dict:
        if num_questions < 1:
            raise ValueError("num_questions must be at least 1")
        if num_questions > settings["ai_max_quiz_questions"]:
            raise ValueError(f"num_questions must be <= {settings['ai_max_quiz_questions']}")
        difficulty = difficulty.lower()
        if difficulty not in {"easy", "medium", "hard"}:
            raise ValueError("difficulty must be easy, medium, or hard")
        context = ContextService.select_context([source_text], "quiz", max_chars=settings["ai_max_context_chars"])
        messages = [
            {"role": "system", "content": PromptService.quiz_system_prompt()},
            {"role": "user", "content": f"Create {num_questions} {difficulty}-difficulty quiz questions grounded in the following material.\n\n{context}"},
        ]
        result = await self.provider.generate_response(messages=messages, context=context, response_format="json")
        payload = result.get("content")
        if not isinstance(payload, dict):
            raise TypeError("malformed quiz payload")

        items = payload.get("questions") or []
        if not isinstance(items, list) or not items:
            raise ValueError("quiz generation returned no questions")

        normalized_items = []
        for item in items[:num_questions]:
            question = item.get("question") if isinstance(item, dict) else None
            options = item.get("options") if isinstance(item, dict) else None
            answer = item.get("correct_answer") or item.get("answer") if isinstance(item, dict) else None
            explanation = item.get("explanation") if isinstance(item, dict) else None
            if not question or not isinstance(options, list) or len(options) < 2 or not answer:
                continue
            normalized_items.append({
                "question": question,
                "options": options,
                "correct_answer": answer,
                "explanation": explanation or "Answer derived from the supplied context.",
                "source_reference": item.get("source_reference") or source_reference,
            })
        if not normalized_items:
            raise ValueError("generated quiz questions are invalid")
        return {"questions": normalized_items, "provider": result.get("metadata", {}).get("provider"), "model": result.get("metadata", {}).get("model")}
