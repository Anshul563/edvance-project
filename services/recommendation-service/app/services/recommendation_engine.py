from __future__ import annotations

from collections.abc import Iterable
from typing import Any


def normalize_score(values: Iterable[float]) -> list[float]:
    values_list = list(values)
    if not values_list:
        return []
    minimum = min(values_list)
    maximum = max(values_list)
    if maximum == minimum:
        return [1.0 if value == maximum and maximum > 0 else 0.0 for value in values_list]
    return [(value - minimum) / (maximum - minimum) for value in values_list]


def _cold_start_interest_score(item: dict[str, Any], user_profile: dict[str, Any] | None) -> float:
    if not user_profile:
        return 0.15
    category = str(item.get("category") or "").lower()
    tags = {str(tag).lower() for tag in item.get("tags") or []}
    profile_categories = {str(value).lower() for value in user_profile.get("categories", [])}
    profile_tags = {str(value).lower() for value in user_profile.get("tags", [])}
    if category in profile_categories:
        return 0.7
    if tags & profile_tags:
        return 0.6
    return 0.15


def rank_candidates(
    items: list[dict[str, Any]],
    weights: dict[str, float] | None = None,
    user_profile: dict[str, Any] | None = None,
) -> list[dict[str, Any]]:
    config = {
        "content": 0.4,
        "interest": 0.25,
        "popularity": 0.2,
        "freshness": 0.15,
    }
    if weights:
        config.update(weights)

    scored: list[dict[str, Any]] = []
    seen_creators: dict[str, int] = {}
    seen_categories: dict[str, int] = {}

    for item in items:
        content_score = float(item.get("content_score", 0.0))
        interest_score = float(item.get("interest_score", 0.0))
        if interest_score <= 0.0:
            interest_score = _cold_start_interest_score(item, user_profile)
        popularity_score = float(item.get("popularity_score", 0.0))
        freshness_score = float(item.get("freshness_score", 0.0))

        score = (
            config["content"] * content_score
            + config["interest"] * interest_score
            + config["popularity"] * popularity_score
            + config["freshness"] * freshness_score
        )
        score = max(0.0, min(1.0, score))

        creator_id = str(item.get("creator_id") or "")
        category = str(item.get("category") or "")
        seen_creators[creator_id] = seen_creators.get(creator_id, 0) + 1
        seen_categories[category] = seen_categories.get(category, 0) + 1

        if creator_id and seen_creators.get(creator_id, 0) > 1:
            score *= 0.9
        if category and seen_categories.get(category, 0) > 2:
            score *= 0.92

        reason = "Matches your interests"
        if interest_score < 0.15 and popularity_score > 0.6:
            reason = "Popular in your category"
        elif freshness_score > 0.8:
            reason = "Recently published"
        elif content_score > 0.7:
            reason = "Similar to content you viewed"

        scored.append(
            {
                "id": item.get("id"),
                "source_type": item.get("source_type"),
                "source_id": item.get("source_id"),
                "title": item.get("title"),
                "description": item.get("description"),
                "category": item.get("category"),
                "score": round(max(0.0, min(1.0, score)), 4),
                "reason": reason,
                "creator_id": creator_id,
                "category_count": seen_categories.get(category, 0),
            }
        )

    scored.sort(key=lambda entry: (-float(entry["score"]), str(entry["title"] or "")))
    return scored
