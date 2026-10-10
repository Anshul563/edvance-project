from __future__ import annotations

import os
from functools import lru_cache


@lru_cache(maxsize=1)
def get_settings() -> dict:
    return {
        "app_env": os.getenv("APP_ENV", "development"),
        "port": int(os.getenv("AI_SERVICE_PORT", "8095")),
        "database_url": os.getenv(
            "DATABASE_URL",
            "postgresql+asyncpg://postgres:postgres@localhost:5432/edvance_ai",
        ),
        "jwt_access_secret": os.getenv("JWT_ACCESS_SECRET", ""),
        "internal_service_token": os.getenv("INTERNAL_SERVICE_TOKEN", ""),
        "api_gateway_url": os.getenv("API_GATEWAY_URL", "http://localhost:8080"),
        "course_service_url": os.getenv("COURSE_SERVICE_URL", "http://localhost:8086"),
        "content_service_url": os.getenv("CONTENT_SERVICE_URL", "http://localhost:8084"),
        "video_service_url": os.getenv("VIDEO_SERVICE_URL", "http://localhost:8085"),
        "learning_service_url": os.getenv("LEARNING_SERVICE_URL", "http://localhost:8087"),
        "ai_provider": os.getenv("AI_PROVIDER", "mock"),
        "ai_model": os.getenv("AI_MODEL", "mock-ai-model"),
        "ai_api_key": os.getenv("AI_API_KEY", ""),
        "ai_base_url": os.getenv("AI_BASE_URL", ""),
        "ai_request_timeout_seconds": float(os.getenv("AI_REQUEST_TIMEOUT_SECONDS", "30")),
        "ai_max_output_tokens": int(os.getenv("AI_MAX_OUTPUT_TOKENS", "2000")),
        "ai_max_context_chars": int(os.getenv("AI_MAX_CONTEXT_CHARS", "24000")),
        "ai_max_quiz_questions": int(os.getenv("AI_MAX_QUIZ_QUESTIONS", "20")),
        "ai_max_requests_per_minute": int(os.getenv("AI_MAX_REQUESTS_PER_MINUTE", "10")),
        "max_question_length": int(os.getenv("AI_MAX_QUESTION_LENGTH", "1000")),
        "max_summary_chars": int(os.getenv("AI_MAX_SUMMARY_CHARS", "4000")),
    }
