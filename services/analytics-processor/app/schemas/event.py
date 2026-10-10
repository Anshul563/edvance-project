from __future__ import annotations

from datetime import datetime
from typing import Any

from pydantic import BaseModel, Field


class AnalyticsEvent(BaseModel):
    event_id: str = Field(..., min_length=36, max_length=64)
    event_type: str
    event_version: int = 1
    occurred_at: datetime
    actor_id: str | None = None
    anonymous_id: str | None = None
    session_id: str | None = None
    source_service: str
    entity_type: str
    entity_id: str
    properties: dict[str, Any] = Field(default_factory=dict)
    schema_version: str = "v1"

    model_config = {"extra": "allow"}

    @property
    def normalized_event_type(self) -> str:
        return self.event_type.strip().lower()
