import os
from functools import lru_cache


@lru_cache(maxsize=1)
def get_settings() -> dict:
    return {
        "app_env": os.getenv("APP_ENV", "development"),
        "port": int(os.getenv("RECOMMENDATION_SERVICE_PORT", "8093")),
        "database_url": os.getenv(
            "DATABASE_URL",
            "postgresql+asyncpg://postgres:postgres@localhost:5432/edvance_recommendation",
        ),
        "jwt_access_secret": os.getenv("JWT_ACCESS_SECRET", ""),
        "internal_service_token": os.getenv("INTERNAL_SERVICE_TOKEN", ""),
        "search_service_url": os.getenv("SEARCH_SERVICE_URL", "http://localhost:8091"),
        "learning_service_url": os.getenv("LEARNING_SERVICE_URL", "http://localhost:8087"),
        "social_service_url": os.getenv("SOCIAL_SERVICE_URL", "http://localhost:8088"),
        "content_service_url": os.getenv("CONTENT_SERVICE_URL", "http://localhost:8084"),
        "default_page_size": int(os.getenv("DEFAULT_PAGE_SIZE", "20")),
        "max_page_size": int(os.getenv("MAX_PAGE_SIZE", "50")),
        "http_client_timeout_seconds": float(os.getenv("HTTP_CLIENT_TIMEOUT_SECONDS", "3")),
        "content_score_weight": float(os.getenv("CONTENT_SCORE_WEIGHT", "0.40")),
        "interest_score_weight": float(os.getenv("INTEREST_SCORE_WEIGHT", "0.25")),
        "popularity_score_weight": float(os.getenv("POPULARITY_SCORE_WEIGHT", "0.20")),
        "freshness_score_weight": float(os.getenv("FRESHNESS_SCORE_WEIGHT", "0.15")),
    }
