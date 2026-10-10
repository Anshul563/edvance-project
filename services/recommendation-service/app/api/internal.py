from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, status

from app.db.session import AsyncSessionLocal
from app.dependencies import require_internal_token
from app.repositories.recommendation_repository import RecommendationRepository

router = APIRouter()


@router.put("/items/{source_type}/{source_id}")
async def upsert_item(source_type: str, source_id: str, _: str = Depends(require_internal_token)):
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        payload = {
            "id": f"{source_type}:{source_id}",
            "source_type": source_type,
            "source_id": source_id,
            "title": "Indexed item",
            "description": "Recorded through internal ingestion",
            "category": "General",
            "tags": [],
            "visibility": "public",
            "is_eligible": True,
            "engagement_score": 0.0,
        }
        await repo.upsert_item(payload)
        await session.commit()
    return {"status": "upserted", "sourceType": source_type, "sourceId": source_id}


@router.delete("/items/{source_type}/{source_id}")
async def delete_item(source_type: str, source_id: str, _: str = Depends(require_internal_token)):
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        deleted = await repo.delete_item(source_type, source_id)
        await session.commit()
    if not deleted:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="item not found")
    return {"status": "deleted", "sourceType": source_type, "sourceId": source_id}
