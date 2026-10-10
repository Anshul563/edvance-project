import pytest

from app.services.recommendation_engine import normalize_score, rank_candidates


@pytest.mark.parametrize(
    ("values", "expected"),
    [
        ([0.1, 0.2, 0.9], [0.0, 0.125, 1.0]),
        ([1, 1, 1], [1.0, 1.0, 1.0]),
        ([0.0, 0.0, 0.0], [0.0, 0.0, 0.0]),
    ],
)
def test_normalize_score(values, expected):
    normalized = normalize_score(values)
    assert normalized == pytest.approx(expected)


def test_rank_candidates_uses_weights_and_diversity():
    items = [
        {
            "id": "a",
            "source_type": "course",
            "source_id": "1",
            "category": "Programming",
            "tags": ["python", "fastapi"],
            "title": "Python API Mastery",
            "content_score": 0.9,
            "interest_score": 0.8,
            "popularity_score": 0.5,
            "freshness_score": 0.6,
            "creator_id": "creator-1",
        },
        {
            "id": "b",
            "source_type": "course",
            "source_id": "2",
            "category": "Programming",
            "tags": ["python"],
            "title": "Python Basics",
            "content_score": 0.88,
            "interest_score": 0.2,
            "popularity_score": 0.7,
            "freshness_score": 0.8,
            "creator_id": "creator-1",
        },
        {
            "id": "c",
            "source_type": "video",
            "source_id": "3",
            "category": "Design",
            "tags": ["ui"],
            "title": "UX Design",
            "content_score": 0.6,
            "interest_score": 0.1,
            "popularity_score": 0.3,
            "freshness_score": 0.9,
            "creator_id": "creator-2",
        },
    ]

    ranked = rank_candidates(items)
    assert ranked[0]["id"] == "a"
    assert all(item["score"] <= 1.0 for item in ranked)
    assert all(item["score"] >= 0.0 for item in ranked)


def test_normalize_items_uses_user_profile_matches():
    from app.api.recommendations import normalize_items

    items = [
        {
            "id": "match",
            "source_type": "course",
            "source_id": "1",
            "title": "FastAPI patterns",
            "description": "Advanced API design",
            "category": "Programming",
            "tags": ["fastapi", "backend"],
            "engagement_score": 40,
            "creator_id": "creator-1",
        },
        {
            "id": "other",
            "source_type": "video",
            "source_id": "2",
            "title": "Brand strategy",
            "description": "Marketing basics",
            "category": "Marketing",
            "tags": ["seo"],
            "engagement_score": 50,
            "creator_id": "creator-2",
        },
    ]

    normalized = normalize_items(items, user_profile={"categories": ["Programming"], "tags": ["fastapi"]})
    assert normalized[0]["interest_score"] > normalized[1]["interest_score"]
    assert normalized[0]["category"] == "Programming"
