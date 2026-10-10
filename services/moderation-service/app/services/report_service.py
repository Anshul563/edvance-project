from __future__ import annotations

from datetime import UTC, datetime

from fastapi import HTTPException, status

from app.repositories.report_repository import ReportRepository

VALID_TARGET_TYPES = {"course", "video", "short", "comment", "post", "creator"}
VALID_REASONS = {
    "spam",
    "harassment",
    "hate",
    "sexual_content",
    "violence",
    "copyright_concern",
    "misleading_content",
    "scam",
    "other",
}


class ReportService:
    def __init__(self, session):
        self.session = session
        self.repository = ReportRepository(session)

    async def submit_report(
        self,
        reporter_user_id: str,
        target_type: str,
        target_id: str,
        reason: str,
        description: str,
    ) -> dict:
        target_type = str(target_type).strip().lower()
        reason = str(reason).strip().lower()
        if target_type not in VALID_TARGET_TYPES:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST, detail="unsupported target type"
            )
        if reason not in VALID_REASONS:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST, detail="unsupported reason"
            )
        if not target_id or not target_id.strip():
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST, detail="target_id is required"
            )
        if len(description.strip()) < 10:
            raise HTTPException(
                status_code=status.HTTP_400_BAD_REQUEST,
                detail="description must be at least 10 characters",
            )
        if await self.repository.exists_recent_duplicate(
            reporter_user_id,
            target_type,
            target_id,
            reason,
        ):
            raise HTTPException(
                status_code=status.HTTP_409_CONFLICT,
                detail="duplicate report already exists",
            )
        report = await self.repository.create(
            {
                "reporter_user_id": reporter_user_id,
                "target_type": target_type,
                "target_id": target_id,
                "reason": reason,
                "description": description.strip(),
                "status": "open",
                "created_at": datetime.now(UTC),
                "updated_at": datetime.now(UTC),
            }
        )
        return {
            "id": report.id,
            "reporter_user_id": report.reporter_user_id,
            "target_type": report.target_type,
            "target_id": report.target_id,
            "reason": report.reason,
            "description": report.description,
            "status": report.status,
            "created_at": report.created_at.isoformat(),
            "updated_at": report.updated_at.isoformat(),
        }

    async def list_reports_for_user(self, user_id: str, offset: int = 0, limit: int = 20):
        return await self.repository.list_for_user(user_id, offset=offset, limit=limit)
