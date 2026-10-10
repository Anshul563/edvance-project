from __future__ import annotations

from pydantic import BaseModel, Field


class InterestUpdateRequest(BaseModel):
    categories: list[str] = Field(default_factory=list, max_length=30)
    tags: list[str] = Field(default_factory=list, max_length=50)


class InterestResponse(BaseModel):
    categories: list[str]
    tags: list[str]
