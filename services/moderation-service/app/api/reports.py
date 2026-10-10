from __future__ import annotations

from fastapi import APIRouter, Header, HTTPException, Query, status

from app.db.session import AsyncSessionLocal
from app.dependencies import get_token_claims, require_authentication
from app.repositories.case_repository import CaseRepository
from app.repositories.report_repository import ReportRepository
from app.schemas.report import ReportCreateRequest, ReportListResponse, ReportResponse
from app.services.report_service import ReportService

router = APIRouter()


@router.post("/reports", response_model=ReportResponse, status_code=status.HTTP_201_CREATED)
async def submit_report(
    payload: ReportCreateRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    reporter_user_id = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        service = ReportService(session)
        report_data = await service.submit_report(
            reporter_user_id=reporter_user_id,
            target_type=payload.target_type,
            target_id=payload.target_id,
            reason=payload.reason,
            description=payload.description,
        )
        case_repo = CaseRepository(session)
        await case_repo.create_for_target(
            report_data["target_type"],
            report_data["target_id"],
            "v1",
        )
        await session.commit()
        return ReportResponse(**report_data)


@router.get("/reports/me")
async def list_my_reports(
    page: int = Query(default=1, ge=1),
    limit: int = Query(default=20, ge=1, le=100),
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        repo = ReportRepository(session)
        offset = (page - 1) * limit
        items = await repo.list_for_user(user_id, offset=offset, limit=limit)
        response_items = [
            ReportResponse(
                id=item.id,
                reporter_user_id=item.reporter_user_id,
                target_type=item.target_type,
                target_id=item.target_id,
                reason=item.reason,
                description=item.description,
                status=item.status,
                created_at=item.created_at.isoformat(),
                updated_at=item.updated_at.isoformat(),
            )
            for item in items
        ]
        return ReportListResponse(
            items=response_items,
            page=page,
            limit=limit,
            has_next=len(response_items) == limit,
        )


@router.get("/reports/{report_id}")
async def get_report(
    report_id: str,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    requester = require_authentication(authorization)
    async with AsyncSessionLocal() as session:
        repo = ReportRepository(session)
        report = await repo.get_by_id(report_id)
        if not report:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="report not found")
        claims = get_token_claims(authorization)
        roles = claims.get("roles") or []
        if isinstance(roles, str):
            roles = [roles]
        role = str(claims.get("role") or "").lower()
        can_view_all = "moderator" in {str(item).lower() for item in roles} or role in {
            "moderator",
            "admin",
        }
        if report.reporter_user_id != requester and not can_view_all:
            raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN, detail="report access denied"
            )
        return ReportResponse(
            id=report.id,
            reporter_user_id=report.reporter_user_id,
            target_type=report.target_type,
            target_id=report.target_id,
            reason=report.reason,
            description=report.description,
            status=report.status,
            created_at=report.created_at.isoformat(),
            updated_at=report.updated_at.isoformat(),
        )
