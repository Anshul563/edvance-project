from __future__ import annotations

from app.config import get_settings
from app.providers.configured_provider import ConfiguredProvider
from app.services.context_service import ContextService
from app.services.prompt_service import PromptService

settings = get_settings()


class SummaryService:
    def __init__(self):
        self.provider = ConfiguredProvider()

    async def summarize(self, *, text: str | None = None, source: str | None = None) -> dict:
        normalized = ContextService.normalize_text(text)
        if not normalized:
            raise ValueError("text is required")
        context = ContextService.select_context([normalized], "summary", max_chars=settings["ai_max_context_chars"])
        messages = [
            {"role": "system", "content": PromptService.summary_system_prompt()},
            {"role": "user", "content": f"Source: {source or 'lesson'}\n\nContent:\n{context}"},
        ]
        result = await self.provider.generate_response(messages=messages, context=context, response_format="json")
        payload = result.get("content") if isinstance(result.get("content"), dict) else {"summary": result.get("content", "")}
        return {
            "summary": payload.get("summary") or payload.get("answer") or "Summary unavailable.",
            "key_takeaways": payload.get("key_takeaways") or [],
            "source_reference": source or "provided-text",
            "provider": result.get("metadata", {}).get("provider"),
            "model": result.get("metadata", {}).get("model"),
        }
