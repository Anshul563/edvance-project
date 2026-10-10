from __future__ import annotations

import asyncio
import os

import jwt
from fastapi.testclient import TestClient

os.environ.setdefault("JWT_ACCESS_SECRET", "test-secret")
os.environ.setdefault("DATABASE_URL", "sqlite+aiosqlite:///:memory:")

from app.config import get_settings
from app.main import app
from app.services.tutor_service import TutorService


def test_health_endpoint():
    with TestClient(app) as client:
        response = client.get("/health")
    assert response.status_code == 200
    assert response.json()["status"] == "ok"


def test_tutor_service_handles_question():
    result = asyncio.run(
        TutorService().ask_question(
            user_id="user-123",
            question="What is the main concept in this lesson?",
            course_id="course-42",
            lesson_id="lesson-99",
        )
    )

    assert result["answer"]
    assert isinstance(result["grounded"], bool)


def test_auth_required_for_ai_endpoint():
    with TestClient(app) as client:
        response = client.post("/api/v1/ai/ask", json={"question": "Explain the lesson"})
    assert response.status_code == 401


def test_valid_token_is_accepted_by_auth_dependency():
    settings = get_settings()
    secret = settings["jwt_access_secret"] or "test-secret"
    token = jwt.encode({"sub": "user-123"}, secret, algorithm="HS256")
    with TestClient(app) as client:
        response = client.post(
            "/api/v1/ai/ask",
            json={"question": "What is covered in this lesson?"},
            headers={"Authorization": f"Bearer {token}"},
        )
    assert response.status_code in {200, 422}
