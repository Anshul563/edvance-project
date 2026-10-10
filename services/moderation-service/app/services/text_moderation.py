from __future__ import annotations

import re
from dataclasses import dataclass


@dataclass
class ModerationResult:
    decision: str
    severity: str
    category_scores: dict[str, float]
    matched_codes: list[str]
    explanation: str
    requires_human_review: bool


class TextModerationService:
    def __init__(self):
        self.patterns = {
            "spam": [
                "click here",
                "limited time",
                "guaranteed",
                "make money fast",
                "buy now",
                "dm me",
                "free money",
                "crypto signal",
                "promo code",
                "affiliate",
            ],
            "harassment": [
                "idiot",
                "moron",
                "stupid",
                "loser",
                "hate you",
                "shut up",
                "you are trash",
            ],
            "hate": [
                "kill all",
                "inferior race",
                "hate group",
                "worthless people",
            ],
            "sexual": [
                "nude",
                "explicit sex",
                "sexual content",
                "horny",
                "naked",
            ],
            "violence": [
                "kill them",
                "bomb",
                "attack now",
                "stab",
                "shoot up",
                "hurt someone",
            ],
            "scam": [
                "click this link",
                "send crypto",
                "free gift card",
                "urgent payout",
                "bank login",
                "verify account now",
            ],
        }

    def analyze(self, text: str) -> ModerationResult:
        normalized = re.sub(r"\s+", " ", (text or "")).strip().lower()
        if not normalized:
            return ModerationResult(
                decision="safe",
                severity="low",
                category_scores={"safe": 1.0},
                matched_codes=[],
                explanation="Empty content is considered safe.",
                requires_human_review=False,
            )

        matched_codes: list[str] = []
        category_scores = {"safe": 0.0, "needs_review": 0.0, "high_risk": 0.0}

        for category, phrases in self.patterns.items():
            hits = [phrase for phrase in phrases if phrase in normalized]
            if hits:
                matched_codes.append(f"{category}:{len(hits)}")
                category_scores["needs_review"] += 0.25
                if category in {"hate", "violence", "sexual"}:
                    category_scores["high_risk"] += 0.3
                if category in {"scam", "spam"}:
                    category_scores["needs_review"] += 0.15

        if re.search(r"(?:https?://|www\.)", normalized):
            matched_codes.append("external_link")
            category_scores["needs_review"] += 0.1

        if re.search(r"\b(?:!!!|\b[A-Z]{3,}\b)\b", text):
            matched_codes.append("urgency")
            category_scores["needs_review"] += 0.08

        category_scores["safe"] = max(
            0.0, 1.0 - (category_scores["needs_review"] + category_scores["high_risk"])
        )
        if category_scores["high_risk"] > 0.5:
            decision = "high_risk"
            severity = "high"
            requires_human_review = True
        elif category_scores["needs_review"] > 0.35 or matched_codes:
            decision = "needs_review"
            severity = "medium" if category_scores["needs_review"] < 0.6 else "high"
            requires_human_review = True
        else:
            decision = "safe"
            severity = "low"
            requires_human_review = False

        explanation = (
            "No harmful pattern detected."
            if not matched_codes
            else f"Detected heuristic risk signals: {', '.join(sorted(set(matched_codes)))}."
        )
        return ModerationResult(
            decision=decision,
            severity=severity,
            category_scores={
                key: round(value, 3)
                for key, value in category_scores.items()
                if value > 0 or key == "safe"
            },
            matched_codes=sorted(set(matched_codes)),
            explanation=explanation,
            requires_human_review=requires_human_review,
        )
