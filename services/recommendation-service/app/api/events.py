from __future__ import annotations

from fastapi import APIRouter, Header, HTTPException, status

from app.db.session import AsyncSessionLocal
from app.dependencies import get_current_user_id
from app.repositories.event_repository import EventRepository
from app.schemas.event import VALID_EVENT_TYPES, EventIngestionRequest

router = APIRouter()


@router.post("/events")
async def ingest_event(
    payload: EventIngestionRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = get_current_user_id(authorization)
    if payload.eventType not in VALID_EVENT_TYPES:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail="unsupported event type"
        )
    if payload.eventWeight < 0 or payload.eventWeight > 10:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST, detail="eventWeight must be between 0 and 10"
        )
    async with AsyncSessionLocal() as session:
        repo = EventRepository(session)
        stored = {
            "user_id": user_id or payload.userId,
            "anonymous_id": payload.anonymousId,
            "source_type": payload.sourceType,
            "source_id": payload.sourceId,
            "event_type": payload.eventType,
            "event_weight": payload.eventWeight,
            "session_id": payload.sessionId,
        }
        await repo.add_event(stored)
        await session.commit()
    return {"status": "accepted", "eventType": payload.eventType}
