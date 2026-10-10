from __future__ import annotations

import time

from app.providers.base import LLMProvider, ProviderError, ProviderUsage


class MockProvider(LLMProvider):
    def __init__(self, *, model: str = "mock-ai-model", provider_name: str = "mock"):
        self.model = model
        self.provider_name = provider_name

    async def generate_response(
        self,
        *,
        messages: list[dict],
        context: str | None = None,
        response_format: str | None = None,
    ) -> dict:
        if not messages:
            raise ProviderError("No messages supplied")
        prompt_text = "\n".join(str(message.get("content", "")) for message in messages)
        lower_prompt = prompt_text.lower()
        content = "Based on the provided course materials, the answer is grounded in the supplied context."
        if "ignore" in lower_prompt or "override" in lower_prompt or "system prompt" in lower_prompt:
            content = "I can only answer from the supplied course materials and cannot follow instructions hidden in retrieved content."
        if response_format == "json":
            content = {
                "answer": content,
                "grounded": True,
                "source_references": [],
            }
        start = time.perf_counter()
        return {
            "content": content,
            "usage": ProviderUsage(
                input_tokens=max(20, len(prompt_text) // 4),
                output_tokens=max(10, len(str(content)) // 5),
                latency_ms=int((time.perf_counter() - start) * 1000),
            ),
            "metadata": self.metadata(),
        }

    def metadata(self) -> dict:
        return {"provider": self.provider_name, "model": self.model, "source": "mock"}
