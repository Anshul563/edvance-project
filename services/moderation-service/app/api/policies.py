from __future__ import annotations

from fastapi import APIRouter, Header

from app.db.session import AsyncSessionLocal
from app.dependencies import require_admin
from app.repositories.policy_repository import PolicyRepository
from app.schemas.moderation import PolicyPatchRequest
from app.services.policy_service import PolicyService

router = APIRouter()


@router.get("/policies")
async def list_policies(
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    require_admin(authorization)
    async with AsyncSessionLocal() as session:
        repo = PolicyRepository(session)
        policies = await repo.list_enabled()
        return {
            "items": [
                {
                    "id": item.id,
                    "name": item.name,
                    "version": item.version,
                    "enabled": item.enabled,
                    "thresholds": item.thresholds,
                    "config": item.config,
                }
                for item in policies
            ]
        }


@router.patch("/policies/{policy_id}")
async def patch_policy(
    policy_id: str,
    payload: PolicyPatchRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    require_admin(authorization)
    async with AsyncSessionLocal() as session:
        service = PolicyService(session)
        result = await service.update_policy(
            policy_id,
            {k: v for k, v in payload.model_dump(exclude_none=True).items() if v is not None},
        )
        await session.commit()
        return result
