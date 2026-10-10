# Edvance Moderation Service

This service manages user-submitted reports, moderation review queues, policies, and deterministic text moderation checks for Edvance content.

## Quick start

```bash
cd /home/anshulll84/Documents/Edvance/services/moderation-service
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -U pip
python -m pip install -e '.[dev]'
cp .env.example .env
createdb edvance_moderation || true
alembic upgrade head
uvicorn app.main:app --host 0.0.0.0 --port 8094 --reload
```

## Environment

Set `JWT_ACCESS_SECRET` and `INTERNAL_SERVICE_TOKEN` before running the service. The default `DATABASE_URL` points at PostgreSQL `edvance_moderation`.

## Public API

- `GET /health`
- `GET /ready`
- `POST /api/v1/moderation/reports`
- `GET /api/v1/moderation/reports/me`
- `GET /api/v1/moderation/reports/{report_id}`
- `POST /api/v1/moderation/assess-text`
- `GET /api/v1/moderation/queue`
- `GET /api/v1/moderation/cases/{case_id}`
- `POST /api/v1/moderation/cases/{case_id}/assign`
- `POST /api/v1/moderation/cases/{case_id}/review`
- `POST /api/v1/moderation/cases/{case_id}/actions`
- `POST /api/v1/moderation/cases/{case_id}/resolve`
- `GET /api/v1/moderation/policies`
- `PATCH /api/v1/moderation/policies/{policy_id}`

## Internal API

- `POST /internal/v1/moderation/assess`
- `POST /internal/v1/moderation/actions/{action_id}/status`

These internal routes require the shared bearer token configured in `INTERNAL_SERVICE_TOKEN`.

## Notes

- The service stores moderation references and snapshots only; it does not own course, user, or payment source-of-truth tables.
- Text moderation is a deterministic rules engine with provider-level abstractions for future replacement.
- Enforcement integrations are represented as pending actions until a trusted upstream service confirms completion.
