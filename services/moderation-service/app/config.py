from __future__ import annotations

import os
from functools import lru_cache


@lru_cache(maxsize=1)
def get_settings() -> dict:
    return {
        "app_env": os.getenv("APP_ENV", "development"),
        "port": int(os.getenv("MODERATION_SERVICE_PORT", "8094")),
        "database_url": os.getenv(
            "DATABASE_URL",
            "postgresql+asyncpg://postgres:postgres@localhost:5432/edvance_moderation",
        ),
        "jwt_access_secret": os.getenv("JWT_ACCESS_SECRET", ""),
        "internal_service_token": os.getenv("INTERNAL_SERVICE_TOKEN", ""),
        "gateway_url": os.getenv("API_GATEWAY_URL", "http://localhost:8080"),
        "content_service_url": os.getenv("CONTENT_SERVICE_URL", "http://localhost:8084"),
        "creator_service_url": os.getenv("CREATOR_SERVICE_URL", "http://localhost:8083"),
        "video_service_url": os.getenv("VIDEO_SERVICE_URL", "http://localhost:8085"),
        "course_service_url": os.getenv("COURSE_SERVICE_URL", "http://localhost:8086"),
        "social_service_url": os.getenv("SOCIAL_SERVICE_URL", "http://localhost:8088"),
        "request_timeout_seconds": float(os.getenv("REQUEST_TIMEOUT_SECONDS", "5")),
        "max_text_length": int(os.getenv("MAX_TEXT_LENGTH", "20000")),
        "default_page_size": int(os.getenv("DEFAULT_PAGE_SIZE", "20")),
        "max_page_size": int(os.getenv("MAX_PAGE_SIZE", "100")),
    }
