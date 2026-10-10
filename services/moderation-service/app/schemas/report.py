from __future__ import annotations

from pydantic import BaseModel, Field, field_validator


class ReportCreateRequest(BaseModel):
    target_type: str = Field(..., min_length=1, max_length=32)
    target_id: str = Field(..., min_length=1, max_length=64)
    reason: str = Field(..., min_length=1, max_length=64)
    description: str = Field(..., min_length=10, max_length=2000)

    @field_validator("target_type")
    @classmethod
    def normalize_target_type(cls, value: str) -> str:
        return value.strip().lower()

    @field_validator("reason")
    @classmethod
    def normalize_reason(cls, value: str) -> str:
        return value.strip().lower()


class ReportResponse(BaseModel):
    id: str
    reporter_user_id: str
    target_type: str
    target_id: str
    reason: str
    description: str
    status: str
    created_at: str
    updated_at: str


class ReportListResponse(BaseModel):
    items: list[ReportResponse]
    page: int
    limit: int
    has_next: bool
