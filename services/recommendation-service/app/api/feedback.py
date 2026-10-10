from __future__ import annotations

from fastapi import APIRouter, Header, HTTPException, status
from sqlalchemy import select

from app.db.session import AsyncSessionLocal
from app.dependencies import get_current_user_id
from app.models.recommendation_feedback import RecommendationFeedback
from app.schemas.event import FeedbackRequest

router = APIRouter()


@router.post("/feedback")
async def ingest_feedback(
    payload: FeedbackRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = get_current_user_id(authorization)
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED, detail="authentication required"
        )
    async with AsyncSessionLocal() as session:
        stmt = select(RecommendationFeedback).where(
            RecommendationFeedback.user_id == user_id,
            RecommendationFeedback.source_type == payload.sourceType,
            RecommendationFeedback.source_id == payload.sourceId,
            RecommendationFeedback.feedback_type == payload.feedbackType,
        )
        existing = await session.execute(stmt)
        if existing.scalars().first():
            return {"status": "already_recorded"}
        feedback = RecommendationFeedback(
            user_id=user_id,
            source_type=payload.sourceType,
            source_id=payload.sourceId,
            feedback_type=payload.feedbackType,
        )
        session.add(feedback)
        await session.commit()
    return {"status": "accepted"}
