from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass


@dataclass
class ProviderUsage:
    input_tokens: int | None = None
    output_tokens: int | None = None
    latency_ms: int | None = None


class ProviderError(RuntimeError):
    pass


class LLMProvider(ABC):
    @abstractmethod
    async def generate_response(
        self,
        *,
        messages: list[dict],
        context: str | None = None,
        response_format: str | None = None,
    ) -> dict:
        raise NotImplementedError

    @abstractmethod
    def metadata(self) -> dict:
        raise NotImplementedError
