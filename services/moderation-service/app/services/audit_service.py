from __future__ import annotations

from app.repositories.audit_repository import AuditRepository


class AuditService:
    def __init__(self, session):
        self.session = session
        self.repository = AuditRepository(session)

    async def log(self, **payload):
        return await self.repository.create(payload)
