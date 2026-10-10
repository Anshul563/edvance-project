# Recommendation Service

This service provides deterministic, testable recommendation ranking for Edvance content using a hybrid heuristic: content similarity, user interest matching, popularity, freshness, and sparse collaborative signals.

## Quick start

```bash
cd /home/anshulll84/Documents/Edvance/services/recommendation-service
python3 -m venv .venv
source .venv/bin/activate
python -m pip install -U pip
python -m pip install -e '.[dev]'
cp .env.example .env
python -m uvicorn app.main:app --host 0.0.0.0 --port 8093 --reload
```

## Environment

The runtime expects PostgreSQL at the configured `DATABASE_URL`. The recommended local database is:

```bash
createdb edvance_recommendation
```

## Public API

- `GET /health`
- `GET /ready`
- `GET /api/v1/recommendations/for-you`
- `GET /api/v1/recommendations/courses`
- `GET /api/v1/recommendations/videos`
- `GET /api/v1/recommendations/similar/{sourceType}/{sourceId}`
- `GET /api/v1/recommendations/trending`
- `GET /api/v1/recommendations/interests`
- `PUT /api/v1/recommendations/interests`
- `POST /api/v1/recommendations/events`
- `POST /api/v1/recommendations/feedback`

## Internal API

Secure ingestion is exposed only on private networking:

- `PUT /internal/v1/recommendations/items/{sourceType}/{sourceId}`
- `DELETE /internal/v1/recommendations/items/{sourceType}/{sourceId}`

These require the shared bearer token from `INTERNAL_SERVICE_TOKEN`.

## Recommendation algorithm

Scores are normalized to a 0..1 range and blended with the configured weights:

```text
content similarity: 0.40
user interests: 0.25
popularity: 0.20
freshness: 0.15
```

The weights are configured from environment variables and the service degrades gracefully when data is sparse or missing. Cold start users receive recommendations from popular public content, onboarding interests, and recent items.
