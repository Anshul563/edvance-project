from __future__ import annotations

from datetime import UTC, datetime

from fastapi import HTTPException, status

from app.repositories.policy_repository import PolicyRepository


class PolicyService:
    def __init__(self, session):
        self.session = session
        self.repository = PolicyRepository(session)

    async def list_policies(self):
        return await self.repository.list_enabled()

    async def update_policy(self, policy_id: str, payload: dict) -> dict:
        policy = await self.repository.get_by_id(policy_id)
        if not policy:
            raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="policy not found")
        update_values = {
            "name": payload.get("name", policy.name),
            "version": payload.get("version", policy.version),
            "enabled": payload.get("enabled", policy.enabled),
            "thresholds": payload.get("thresholds", policy.thresholds),
            "config": payload.get("config", policy.config),
            "updated_at": datetime.now(UTC),
        }
        policy = await self.repository.update(policy, update_values)
        return {
            "id": policy.id,
            "name": policy.name,
            "version": policy.version,
            "enabled": policy.enabled,
            "thresholds": policy.thresholds,
            "config": policy.config,
            "updated_at": policy.updated_at.isoformat(),
        }
