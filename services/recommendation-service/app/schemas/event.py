from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field

VALID_EVENT_TYPES = {
    "impression",
    "click",
    "watch",
    "watch_completed",
    "course_enrolled",
    "course_completed",
    "like",
    "save",
    "dismiss",
}


class EventIngestionRequest(BaseModel):
    anonymousId: str | None = None
    userId: str | None = None
    sourceType: str
    sourceId: str
    eventType: Literal[tuple(sorted(VALID_EVENT_TYPES))] = Field(...)
    eventWeight: float = Field(default=1.0, ge=0.0, le=10.0)
    sessionId: str | None = None


class FeedbackRequest(BaseModel):
    sourceType: str
    sourceId: str
    feedbackType: Literal["not_interested", "already_seen", "irrelevant"]
