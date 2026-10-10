from __future__ import annotations

import time

import httpx

from app.config import get_settings
from app.providers.base import LLMProvider, ProviderError, ProviderUsage
from app.providers.mock import MockProvider

settings = get_settings()


class ConfiguredProvider(LLMProvider):
    def __init__(self):
        self.provider_name = settings["ai_provider"]
        self.model = settings["ai_model"] or "configured-model"
        self.api_key = settings["ai_api_key"]
        self.base_url = settings["ai_base_url"]
        self.timeout = settings["ai_request_timeout_seconds"]
        self.mock = MockProvider(model=self.model, provider_name=self.provider_name)

    async def generate_response(
        self,
        *,
        messages: list[dict],
        context: str | None = None,
        response_format: str | None = None,
    ) -> dict:
        if not self.base_url and not self.api_key:
            return await self.mock.generate_response(messages=messages, context=context, response_format=response_format)

        payload = {"model": self.model, "messages": messages, "context": context, "response_format": response_format}
        if self.api_key:
            payload["api_key"] = self.api_key
        started = time.perf_counter()
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                response = await client.post(self.base_url, json=payload)
                response.raise_for_status()
        except httpx.HTTPError as exc:
            raise ProviderError(f"AI provider unavailable: {exc}") from exc

        try:
            data = response.json()
        except ValueError as exc:
            raise ProviderError("Malformed provider response") from exc

        payload_data = {
            "content": data.get("answer") or data.get("content") or data.get("response") or "No answer available.",
            "usage": ProviderUsage(
                input_tokens=data.get("usage", {}).get("prompt_tokens") if isinstance(data.get("usage"), dict) else None,
                output_tokens=data.get("usage", {}).get("completion_tokens") if isinstance(data.get("usage"), dict) else None,
                latency_ms=int((time.perf_counter() - started) * 1000),
            ),
            "metadata": {"provider": self.provider_name, "model": self.model, "source": self.base_url},
        }
        if response_format == "json":
            if not isinstance(data.get("json") if isinstance(data, dict) else None, dict):
                raise ProviderError("Provider returned malformed JSON")
            payload_data["content"] = data["json"]
        return payload_data

    def metadata(self) -> dict:
        return {"provider": self.provider_name, "model": self.model}
