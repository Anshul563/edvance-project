from __future__ import annotations

from fastapi import APIRouter, Header, Query
from sqlalchemy.ext.asyncio import AsyncSession

from app.config import get_settings
from app.db.session import AsyncSessionLocal
from app.dependencies import get_current_user_id
from app.repositories.interest_repository import InterestRepository
from app.repositories.recommendation_repository import RecommendationRepository
from app.schemas.recommendation import RecommendationItemResponse, RecommendationListResponse
from app.services.recommendation_engine import rank_candidates

router = APIRouter()
settings = get_settings()


async def get_db() -> AsyncSession:
    async with AsyncSessionLocal() as session:
        yield session


def normalize_items(
    items,
    user_id: str | None = None,
    user_profile: dict[str, list[str]] | None = None,
):
    profile_categories = {str(value).lower() for value in (user_profile or {}).get("categories", [])}
    profile_tags = {str(value).lower() for value in (user_profile or {}).get("tags", [])}
    out = []
    for item in items:
        item_data = item if isinstance(item, dict) else {
            "id": getattr(item, "id", None),
            "source_type": getattr(item, "source_type", None),
            "source_id": getattr(item, "source_id", None),
            "title": getattr(item, "title", None),
            "description": getattr(item, "description", None),
            "category": getattr(item, "category", None),
            "tags": getattr(item, "tags", None),
            "engagement_score": getattr(item, "engagement_score", 0.0),
            "creator_id": getattr(item, "creator_id", None),
        }

        score = round(float(item_data.get("engagement_score") or 0.0) / 10.0 if item_data.get("engagement_score") else 0.5, 4)
        category = str(item_data.get("category") or "").lower()
        raw_tags = item_data.get("tags") or []
        tags = {str(tag).lower() for tag in raw_tags if tag}
        if category and category in profile_categories:
            interest_score = 0.8
        elif tags & profile_tags:
            interest_score = 0.6
        elif user_id:
            interest_score = 0.25
        else:
            interest_score = 0.1

        out.append(
            {
                "id": item_data.get("id"),
                "source_type": item_data.get("source_type"),
                "source_id": item_data.get("source_id"),
                "title": item_data.get("title"),
                "description": item_data.get("description"),
                "category": item_data.get("category"),
                "tags": list(tags),
                "content_score": score,
                "interest_score": interest_score,
                "popularity_score": min(1.0, float(item_data.get("engagement_score") or 0.0) / 100.0),
                "freshness_score": 0.7,
                "creator_id": item_data.get("creator_id"),
            }
        )
    return out


@router.get("/for-you")
async def for_you(
    source_type: str = Query(default="all"),
    page: int = Query(default=1, ge=1),
    limit: int = Query(default=20, ge=1, le=50),
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = get_current_user_id(authorization)
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        interest_repo = InterestRepository(session)
        items = await repo.get_by_type(source_type, limit=limit, offset=(page - 1) * limit)
        user_profile = await interest_repo.profile_for_user(user_id) if user_id else None
        candidate_items = normalize_items(items, user_id=user_id, user_profile=user_profile)
        ranked = rank_candidates(candidate_items, user_profile=user_profile)
        response_items = [
            RecommendationItemResponse(
                id=item["id"],
                sourceType=item["source_type"],
                sourceId=item["source_id"],
                title=item["title"],
                description=item["description"],
                category=item["category"],
                score=item["score"],
                reason=item["reason"],
            )
            for item in ranked
        ]
        return RecommendationListResponse(
            items=response_items, page=page, limit=limit, hasNext=len(response_items) == limit
        )


@router.get("/courses")
async def courses(page: int = Query(default=1, ge=1), limit: int = Query(default=20, ge=1, le=50)):
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        items = await repo.get_by_type("course", limit=limit, offset=(page - 1) * limit)
        ranked = rank_candidates(normalize_items(items))
        return {"items": ranked, "page": page, "limit": limit, "hasNext": len(ranked) == limit}


@router.get("/videos")
async def videos(page: int = Query(default=1, ge=1), limit: int = Query(default=20, ge=1, le=50)):
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        items = await repo.get_by_type("video", limit=limit, offset=(page - 1) * limit)
        ranked = rank_candidates(normalize_items(items))
        return {"items": ranked, "page": page, "limit": limit, "hasNext": len(ranked) == limit}


@router.get("/similar/{source_type}/{source_id}")
async def similar(source_type: str, source_id: str):
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        items = await repo.get_similar(source_type, source_id, limit=10)
        ranked = rank_candidates(normalize_items(items))
        return {"items": ranked, "count": len(ranked)}


@router.get("/trending")
async def trending(limit: int = Query(default=20, ge=1, le=50)):
    async with AsyncSessionLocal() as session:
        repo = RecommendationRepository(session)
        items = await repo.get_trending(limit=limit)
        ranked = rank_candidates(normalize_items(items))
        return {"items": ranked, "limit": limit}
