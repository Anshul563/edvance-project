from __future__ import annotations

from fastapi import APIRouter, Header, Query

from app.db.session import AsyncSessionLocal
from app.dependencies import require_moderator
from app.repositories.case_repository import CaseRepository

router = APIRouter()


@router.get("/queue")
async def moderator_queue(
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
        cases = await repo.list_cases(
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
                }
                for item in cases
            ],
            "page": page,
            "limit": limit,
            "has_next": len(cases) == limit,
        }
