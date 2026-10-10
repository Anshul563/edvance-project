from __future__ import annotations

from app.services.text_moderation import TextModerationService


class ModerationEngine:
    def __init__(self):
        self.text_service = TextModerationService()

    def assess_text(self, text: str):
        return self.text_service.analyze(text)
