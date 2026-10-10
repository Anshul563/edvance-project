from __future__ import annotations

from fastapi import APIRouter

from app.api import health

router = APIRouter(prefix="/api/v1")
router.include_router(health.router)
