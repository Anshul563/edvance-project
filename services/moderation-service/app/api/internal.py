from __future__ import annotations

from fastapi import APIRouter, Header

from app.dependencies import require_internal_token
from app.schemas.moderation import TextAssessmentRequest, TextAssessmentResponse
from app.services.moderation_engine import ModerationEngine

router = APIRouter()


@router.post("/assess")
async def internal_assess(
    payload: TextAssessmentRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    require_internal_token(authorization)
    result = ModerationEngine().assess_text(payload.text)
    return TextAssessmentResponse(
        decision=result.decision,
        severity=result.severity,
        category_scores=result.category_scores,
        matched_codes=result.matched_codes,
        explanation=result.explanation,
        requires_human_review=result.requires_human_review,
    )
