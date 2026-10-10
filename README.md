# Edvance monorepo

Edvance is a multi-service learning platform with Go microservices for core business flows and Python services for AI, moderation, and recommendation workloads.

## Repository layout

- `services/` — service modules and service-specific tests
- `docs/architecture/` — architecture notes and service inventory
- `docs/api/` — API documentation and contract references
- `infrastructure/` — local runtime support such as Postgres, Redis, NATS, and monitoring folders
- `packages/` — shared contracts, events, and protocol definitions
- `apps/` — application UIs or tooling if present

## Prerequisites

- Go 1.26.5
- Python 3.12+ or the project-local virtualenvs used by the Python services
- Postgres running locally for service databases
- Redis and NATS if you are testing the services that depend on them

## Local environment setup

1. Copy `.env.example` to a local environment file for your shell or your service-specific .env.
2. Set the required environment variables for the service you are starting.
3. Ensure the database URLs point to your local Postgres instance.
4. Keep admin JWT secrets and internal service tokens out of committed files.

## Root commands

Use the root Makefile for consistent developer workflows:

- `make help`
- `make test`
- `make build`
- `make lint`

## Verified checks

The repository currently supports these targeted checks:

- `go test ./services/admin-api/... ./services/api-gateway/...`
- `cd services/moderation-service && ./.venv/bin/python -m pytest -q tests`
- `cd services/recommendation-service && ./.venv/bin/python -m pytest -q tests`

If a service does not have a configured virtualenv or environment, do not force that service to start as part of unrelated validation.

## Architecture notes

- The API gateway is the public entrypoint and proxies to backend services.
- Auth and identity validation are centralized in the Auth Service.
- The Admin API is purposely fail-closed and only permits explicitly configured admin subjects.
- Python services are used for recommendation, moderation, and AI processing logic.
- Back-end ownership should stay consistent with service boundaries; do not read another service's database directly.

## Known gaps

- No Docker Compose stack is present under `infrastructure/docker`.
- Root environment files were previously empty and needed explicit placeholders.
- Some service integrations are implemented but not fully integrated into a single local compose workflow.
- The repo should be kept aligned with real service ownership and actual runtime dependencies.

## Troubleshooting

- If Go module tests fail, check `go.work` and ensure the intended module is included.
- If a service returns 401 or 403, verify the JWT issuer, audience, and admin allowlist.
- If service health checks fail, ensure the service's app port and backing database are running.
- If no Python venv exists, use the service's actual local environment or create one intentionally instead of assuming a shared environment.
