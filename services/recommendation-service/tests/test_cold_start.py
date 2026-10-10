from app.services.recommendation_engine import rank_candidates


def test_cold_start_ranking_uses_popularity_and_interest_fallback():
    items = [
        {
            "id": "a",
            "source_type": "course",
            "source_id": "1",
            "title": "Go APIs",
            "description": "backend development",
            "category": "Programming",
            "tags": ["golang", "backend"],
            "content_score": 0.4,
            "interest_score": 0.0,
            "popularity_score": 0.9,
            "freshness_score": 0.8,
            "creator_id": "creator-1",
        },
        {
            "id": "b",
            "source_type": "video",
            "source_id": "2",
            "title": "Design system",
            "description": "ux basics",
            "category": "Design",
            "tags": ["ui"],
            "content_score": 0.5,
            "interest_score": 0.0,
            "popularity_score": 0.2,
            "freshness_score": 0.7,
            "creator_id": "creator-2",
        },
    ]

    ranked = rank_candidates(items, user_profile={"categories": ["Programming"], "tags": ["golang"]})
    assert ranked[0]["id"] == "a"
    assert ranked[0]["reason"] in {"Matches your interests", "Popular in your category"}
