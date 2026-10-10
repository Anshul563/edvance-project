from __future__ import annotations

from fastapi import APIRouter, Header, HTTPException, Query, status

from app.db.session import AsyncSessionLocal
from app.dependencies import require_authentication, require_moderator
from app.repositories.case_repository import CaseRepository
from app.schemas.moderation import (
    ActionRequest,
    CaseAssignRequest,
    CaseReviewRequest,
    ResolveRequest,
    TextAssessmentRequest,
    TextAssessmentResponse,
)
from app.services.moderation_engine import ModerationEngine
from app.services.review_service import ReviewService

router = APIRouter()


@router.post("/assess-text", response_model=TextAssessmentResponse)
async def assess_text(
    payload: TextAssessmentRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    require_authentication(authorization)
    result = ModerationEngine().assess_text(payload.text)
    return TextAssessmentResponse(
        decision=result.decision,
        severity=result.severity,
        category_scores=result.category_scores,
        matched_codes=result.matched_codes,
        explanation=result.explanation,
        requires_human_review=result.requires_human_review,
    )


@router.get("/queue")
async def queue(
    status: str | None = Query(default=None),
    priority: str | None = Query(default=None),
    target_type: str | None = Query(default=None),
    page: int = Query(default=1, ge=1),
    limit: int = Query(default=20, ge=1, le=100),
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    require_moderator(authorization)
    async with AsyncSessionLocal() as session:
        repo = CaseRepository(session)
        items = await repo.list_cases(
            status=status,
            priority=priority,
            target_type=target_type,
            offset=(page - 1) * limit,
            limit=limit,
        )
        return {
            "items": [
                {
                    "id": item.id,
                    "target_type": item.target_type,
                    "target_id": item.target_id,
                    "status": item.status,
                    "priority": item.priority,
                    "assigned_moderator_id": item.assigned_moderator_id,
                    "automated_severity": item.automated_severity,
                    "policy_version": item.policy_version,
                    "created_at": item.created_at.isoformat(),
                    "updated_at": item.updated_at.isoformat(),
                    "resolved_at": item.resolved_at.isoformat() if item.resolved_at else None,
                }
                for item in items
            ],
            "page": page,
            "limit": limit,
            "has_next": len(items) == limit,
        }


@router.get("/cases/{case_id}")
async def get_case(
    case_id: str,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    require_moderator(authorization)
    async with AsyncSessionLocal() as session:
        repo = CaseRepository(session)
        case = await repo.get_by_id(case_id)
        if not case:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="case not found")
        return {
            "id": case.id,
            "target_type": case.target_type,
            "target_id": case.target_id,
            "status": case.status,
            "priority": case.priority,
            "assigned_moderator_id": case.assigned_moderator_id,
            "automated_severity": case.automated_severity,
            "policy_version": case.policy_version,
            "created_at": case.created_at.isoformat(),
            "updated_at": case.updated_at.isoformat(),
            "resolved_at": case.resolved_at.isoformat() if case.resolved_at else None,
        }


@router.post("/cases/{case_id}/assign")
async def assign_case(
    case_id: str,
    payload: CaseAssignRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    moderator_id = require_moderator(authorization)
    async with AsyncSessionLocal() as session:
        service = ReviewService(session)
        case = await service.assign_case(case_id, payload.moderator_id or moderator_id)
        await session.commit()
        return {
            "case_id": case.id,
            "assigned_moderator_id": case.assigned_moderator_id,
            "status": case.status,
        }


@router.post("/cases/{case_id}/review")
async def review_case(
    case_id: str,
    payload: CaseReviewRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    reviewer_id = require_moderator(authorization)
    async with AsyncSessionLocal() as session:
        service = ReviewService(session)
        result = await service.record_review(
            case_id=case_id,
            reviewer_id=reviewer_id,
            outcome=payload.outcome.lower(),
            reason=payload.reason,
            notes=payload.notes,
        )
        await session.commit()
        return result


@router.post("/cases/{case_id}/actions")
async def create_action(
    case_id: str,
    payload: ActionRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    actor_id = require_moderator(authorization)
    async with AsyncSessionLocal() as session:
        service = ReviewService(session)
        action = await service.create_action(
            case_id=case_id,
            target_type=payload.target_type,
            target_id=payload.target_id,
            action_type=payload.action_type,
            reason=payload.reason,
            actor_type="moderator",
            actor_id=actor_id,
        )
        await session.commit()
        return {
            "id": action.id,
            "case_id": action.case_id,
            "action_type": action.action_type,
            "status": action.status,
        }


@router.post("/cases/{case_id}/resolve")
async def resolve_case(
    case_id: str,
    payload: ResolveRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    actor_id = require_moderator(authorization)
    async with AsyncSessionLocal() as session:
        service = ReviewService(session)
        case = await service.resolve_case(case_id, payload.outcome, payload.reason, actor_id)
        await session.commit()
        return {
            "case_id": case.id,
            "status": case.status,
            "resolved_at": case.resolved_at.isoformat(),
        }
