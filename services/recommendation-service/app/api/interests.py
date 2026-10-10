from __future__ import annotations

from fastapi import APIRouter, Header, HTTPException, status

from app.db.session import AsyncSessionLocal
from app.dependencies import get_current_user_id
from app.repositories.interest_repository import InterestRepository
from app.schemas.interest import InterestResponse, InterestUpdateRequest

router = APIRouter()


@router.get("/interests")
async def get_interests(authorization: str | None = Header(default=None, alias="Authorization")):
    user_id = get_current_user_id(authorization)
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED, detail="authentication required"
        )
    async with AsyncSessionLocal() as session:
        repo = InterestRepository(session)
        interests = await repo.list_for_user(user_id)
        categories = sorted({item.category for item in interests if item.category})
        tags = sorted({item.tag for item in interests if item.tag})
        return InterestResponse(categories=categories, tags=tags)


@router.put("/interests")
async def update_interests(
    payload: InterestUpdateRequest,
    authorization: str | None = Header(default=None, alias="Authorization"),
):
    user_id = get_current_user_id(authorization)
    if not user_id:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED, detail="authentication required"
        )
    categories = [v.strip() for v in payload.categories if v and v.strip()]
    tags = [v.strip().lower() for v in payload.tags if v and v.strip()]
    if len(categories) > 30 or len(tags) > 50:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail="too many interests")
    async with AsyncSessionLocal() as session:
        repo = InterestRepository(session)
        await repo.set_preferences(user_id, categories, tags)
        await session.commit()
        return InterestResponse(categories=categories, tags=tags)
