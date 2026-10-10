from __future__ import annotations

from pydantic import BaseModel, Field


class TextAssessmentRequest(BaseModel):
    text: str = Field(..., min_length=1, max_length=20000)


class TextAssessmentResponse(BaseModel):
    decision: str
    severity: str
    category_scores: dict[str, float]
    matched_codes: list[str]
    explanation: str
    requires_human_review: bool


class CaseAssignRequest(BaseModel):
    moderator_id: str = Field(..., min_length=1, max_length=36)


class CaseReviewRequest(BaseModel):
    outcome: str = Field(..., min_length=1, max_length=32)
    reason: str = Field(..., min_length=1, max_length=200)
    notes: str = Field(default="", max_length=2000)


class ActionRequest(BaseModel):
    action_type: str = Field(..., min_length=1, max_length=32)
    reason: str = Field(..., min_length=1, max_length=500)
    target_type: str = Field(..., min_length=1, max_length=32)
    target_id: str = Field(..., min_length=1, max_length=64)


class ResolveRequest(BaseModel):
    outcome: str = Field(..., min_length=1, max_length=32)
    reason: str = Field(..., min_length=1, max_length=500)


class PolicyPatchRequest(BaseModel):
    name: str | None = Field(default=None, min_length=1, max_length=128)
    version: str | None = Field(default=None, min_length=1, max_length=32)
    enabled: bool | None = None
    thresholds: dict | None = None
    config: dict | None = None
