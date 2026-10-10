from __future__ import annotations

from datetime import UTC, datetime

from fastapi import HTTPException, status

from app.models.moderation_action import ModerationAction
from app.models.moderation_assessment import ModerationAssessment
from app.models.moderation_case import ModerationCase
from app.repositories.action_repository import ActionRepository
from app.repositories.audit_repository import AuditRepository
from app.repositories.case_repository import CaseRepository


class ReviewService:
    def __init__(self, session):
        self.session = session
        self.case_repository = CaseRepository(session)
        self.action_repository = ActionRepository(session)
        self.audit_repository = AuditRepository(session)

    async def create_case_for_report(self, report: dict) -> ModerationCase:
        case = await self.case_repository.create_for_target(
            target_type=report["target_type"],
            target_id=report["target_id"],
            policy_version="v1",
        )
        return case

    async def assign_case(self, case_id: str, moderator_id: str) -> ModerationCase:
        case = await self.case_repository.get_by_id(case_id)
        if not case:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="case not found")
        case.assigned_moderator_id = moderator_id
        case.status = "reviewing"
        case.updated_at = datetime.now(UTC)
        await self.session.flush()
        return case

    async def record_review(
        self,
        case_id: str,
        reviewer_id: str,
        outcome: str,
        reason: str,
        notes: str,
    ) -> dict:
        case = await self.case_repository.get_by_id(case_id)
        if not case:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="case not found")
        if case.status not in {"open", "reviewing"}:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="invalid case state for review",
            )
        assessment = ModerationAssessment(
            case_id=case_id,
            assessment_type="human_review",
            provider="moderator",
            severity="medium",
            decision=outcome,
            reason_codes=[reason],
            category_scores={"human_review": 1.0},
        )
        self.session.add(assessment)
        case.status = "resolved" if outcome in {"dismissed", "resolved"} else "reviewing"
        case.updated_at = datetime.now(UTC)
        if case.status == "resolved":
            case.resolved_at = datetime.now(UTC)
        await self.session.flush()
        await self.audit_repository.create(
            {
                "actor": reviewer_id,
                "action": "reviewed_case",
                "case_id": case_id,
                "target_type": case.target_type,
                "target_id": case.target_id,
                "previous_state": "open",
                "new_state": case.status,
                "metadata": {"outcome": outcome, "reason": reason, "notes": notes},
            }
        )
        return {"assessment_id": assessment.id, "case_status": case.status}

    async def create_action(
        self,
        case_id: str,
        target_type: str,
        target_id: str,
        action_type: str,
        reason: str,
        actor_type: str,
        actor_id: str | None,
    ) -> ModerationAction:
        case = await self.case_repository.get_by_id(case_id)
        if not case:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="case not found")
        action = await self.action_repository.create(
            {
                "case_id": case_id,
                "target_type": target_type,
                "target_id": target_id,
                "action_type": action_type,
                "reason": reason,
                "actor_type": actor_type,
                "actor_id": actor_id,
                "status": "pending",
            }
        )
        return action

    async def resolve_case(
        self, case_id: str, outcome: str, reason: str, actor_id: str
    ) -> ModerationCase:
        case = await self.case_repository.get_by_id(case_id)
        if not case:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="case not found")
        if outcome not in {"resolved", "dismissed", "escalated"}:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST, detail="invalid case outcome"
            )
        previous = case.status
        case.status = outcome
        case.resolved_at = datetime.now(UTC)
        case.updated_at = datetime.now(UTC)
        await self.session.flush()
        await self.audit_repository.create(
            {
                "actor": actor_id,
                "action": "resolved_case",
                "case_id": case_id,
                "target_type": case.target_type,
                "target_id": case.target_id,
                "previous_state": previous,
                "new_state": outcome,
                "metadata": {"reason": reason},
            }
        )
        return case
