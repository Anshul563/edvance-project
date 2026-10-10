from __future__ import annotations

import asyncio
import logging

from sqlalchemy.ext.asyncio import AsyncSession

from app.config import get_settings
from app.db.session import AsyncSessionLocal, initialize_database
from app.services.processor import AnalyticsProcessor

logger = logging.getLogger("analytics-processor")


async def run_worker_once() -> int:
    settings = get_settings()
    await initialize_database()
    async with AsyncSessionLocal() as session:
        processor = AnalyticsProcessor(session)
        processed = await processor.process_batch(limit=settings["batch_size"])
        await session.commit()
        return processed


async def run_worker() -> None:
    settings = get_settings()
    logger.setLevel(settings["log_level"])
    await initialize_database()
    while True:
        async with AsyncSessionLocal() as session:
            processor = AnalyticsProcessor(session)
            processed = await processor.process_batch(limit=settings["batch_size"])
            await session.commit()
            logger.info("processed %s analytics events", processed)
        if settings["process_once"]:
            break
        await asyncio.sleep(settings["worker_interval_seconds"])
