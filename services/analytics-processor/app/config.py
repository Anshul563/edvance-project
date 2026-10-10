from __future__ import annotations

import os
from functools import lru_cache


@lru_cache(maxsize=1)
def get_settings() -> dict:
    return {
        "app_env": os.getenv("APP_ENV", "development"),
        "database_url": os.getenv(
            "DATABASE_URL",
            "postgresql+asyncpg://postgres:postgres@localhost:5432/edvance_analytics",
        ),
        "batch_size": int(os.getenv("BATCH_SIZE", "100")),
        "worker_interval_seconds": int(os.getenv("WORKER_INTERVAL_SECONDS", "30")),
        "max_retries": int(os.getenv("MAX_RETRIES", "5")),
        "log_level": os.getenv("LOG_LEVEL", "INFO"),
        "process_once": os.getenv("PROCESS_ONCE", "false").lower() == "true",
    }
