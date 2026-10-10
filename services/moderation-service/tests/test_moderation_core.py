from __future__ import annotations

import pytest
from fastapi import HTTPException
from sqlalchemy.ext.asyncio import AsyncSession, async_sessionmaker, create_async_engine

from app.db.base import Base
from app.services.report_service import ReportService
from app.services.text_moderation import TextModerationService


@pytest.fixture
async def db_session():
    engine = create_async_engine("sqlite+aiosqlite:///:memory:")
    async with engine.begin() as connection:
        await connection.run_sync(Base.metadata.create_all)

    factory = async_sessionmaker(engine, expire_on_commit=False, class_=AsyncSession)
    async with factory() as session:
        yield session

    await engine.dispose()


def test_text_moderation_detects_risky_language():
    result = TextModerationService().analyze(
        "Click here now and send crypto to claim a free gift card."
    )

    assert result.decision == "needs_review"
    assert result.requires_human_review is True
    assert "spam" in "".join(result.matched_codes) or "scam" in "".join(result.matched_codes)


@pytest.mark.asyncio
async def test_report_service_rejects_duplicate_report(db_session):
    service = ReportService(db_session)

    first = await service.submit_report(
        reporter_user_id="user-123",
        target_type="course",
        target_id="target-123",
        reason="spam",
        description="This course is promoting a scam and suspicious activity.",
    )

    assert first["status"] == "open"

    with pytest.raises(HTTPException, match="duplicate report already exists"):
        await service.submit_report(
            reporter_user_id="user-123",
            target_type="course",
            target_id="target-123",
            reason="spam",
            description="This is a duplicate report with enough description length.",
        )
