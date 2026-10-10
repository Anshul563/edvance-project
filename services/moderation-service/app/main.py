from __future__ import annotations

from fastapi import FastAPI

from app.api import health, internal, moderation, policies, reports
from app.api import review_queue as queue_api

app = FastAPI(title="Edvance Moderation Service", version="0.1.0")

app.include_router(health.router)
app.include_router(reports.router, prefix="/api/v1/moderation")
app.include_router(moderation.router, prefix="/api/v1/moderation")
app.include_router(queue_api.router, prefix="/api/v1/moderation")
app.include_router(policies.router, prefix="/api/v1/moderation")
app.include_router(internal.router, prefix="/internal/v1/moderation")
