from __future__ import annotations

from typing import Literal

from pydantic import BaseModel, Field


class RecommendationItemResponse(BaseModel):
    id: str
    sourceType: str
    sourceId: str
    title: str
    description: str | None = None
    thumbnailUrl: str | None = None
    category: str | None = None
    score: float = 0.0
    reason: str | None = None


class RecommendationListResponse(BaseModel):
    items: list[RecommendationItemResponse]
    page: int = 1
    limit: int = 20
    hasNext: bool = False


class RecommendationQuery(BaseModel):
    type: Literal["all", "course", "video", "short", "post", "creator"] = "all"
    page: int = Field(default=1, ge=1)
    limit: int = Field(default=20, ge=1, le=50)
