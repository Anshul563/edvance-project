from __future__ import annotations

import httpx

from app.config import get_settings

settings = get_settings()


class RetrievalService:
    def __init__(self):
        self.course_service_url = settings["course_service_url"]
        self.content_service_url = settings["content_service_url"]
        self.video_service_url = settings["video_service_url"]
        self.learning_service_url = settings["learning_service_url"]

    async def fetch_course_material(self, *, course_id: str | None = None, lesson_id: str | None = None, token: str | None = None) -> dict:
        data = {"course_id": course_id, "lesson_id": lesson_id, "material": []}
        if lesson_id:
            try:
                async with httpx.AsyncClient(timeout=10.0) as client:
                    headers = {"Authorization": token} if token else None
                    response = await client.get(f"{self.course_service_url}/api/v1/lessons/{lesson_id}", headers=headers)
                    if response.status_code == 200:
                        payload = response.json()
                        lesson_text = payload.get("content") or payload.get("description") or ""
                        if lesson_text:
                            data["material"].append({"source": f"lesson:{lesson_id}", "text": lesson_text})
            except httpx.HTTPError:
                pass
        if course_id and not data["material"]:
            try:
                async with httpx.AsyncClient(timeout=10.0) as client:
                    headers = {"Authorization": token} if token else None
                    response = await client.get(f"{self.course_service_url}/api/v1/courses/{course_id}/structure", headers=headers)
                    if response.status_code == 200:
                        payload = response.json()
                        sections = payload.get("sections") or []
                        for section in sections:
                            for lesson in section.get("lessons", []):
                                lesson_text = lesson.get("content") or lesson.get("summary") or lesson.get("title") or ""
                                if lesson_text:
                                    data["material"].append({"source": f"lesson:{lesson.get('id')}", "text": lesson_text})
            except httpx.HTTPError:
                pass
        if not data["material"]:
            data["material"] = [{"source": "no_material", "text": "No course material was available for grounding."}]
        return data
