# Edvance monorepo

This repository contains a multi-service learning platform with Go services for core business domains and Python services for recommendation and moderation workloads. The local development stack is intentionally conservative: only the services that are implemented and tested are included in the default Docker Compose profile.

## Supported local stack

Default profile:

- PostgreSQL 16
- Redis 7
- API Gateway
- Admin API
- Auth Service
- Analytics Service
- Recommendation Service
- Moderation Service

Optional profile:

- NATS JetStream (`--profile nats`)

Unsupported by default:

- User, Creator, Course, Commerce, Payment, Search, Notification, Media, Video, and Live Streaming services are present in the repo, but they are not included in the default Compose profile because this stack is intentionally limited to services with reliable local startup and known configuration.
- AI service is present but intentionally excluded from the default profile because it is not yet part of the verified default local workflow.

## Prerequisites

- Docker Engine 24+ with Docker Compose V2
- Go 1.26.5 for local module builds and tests
- Python 3.12+ if you want to run the Python services outside a container

## One-time local setup

From the repo root:

```bash
cp .env.example .env
```

The `.env` file is local-only and is ignored by git. Do not commit credentials or production secrets.

## Start infrastructure

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml up -d postgres redis
```

Optional NATS profile:

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml --profile nats up -d nats
```

## Start the default application stack

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml up -d api-gateway admin-api auth-service analytics-service recommendation-service moderation-service
```

These services use Compose DNS hostnames instead of `localhost` so the containers can resolve one another correctly.

## Apply migrations

The local database initialization script creates the required databases under PostgreSQL, but it does not apply service migrations automatically. Apply the migrations explicitly:

```bash
make migrate
```

This runs the SQL migrations for the included Go services and the Alembic migrations for the included Python services.

## Check service status and logs

```bash
make infra-ps
make infra-logs
```

You can also inspect the raw compose config without starting the stack:

```bash
make infra-config
```

## Health checks

The default profile exposes:

- PostgreSQL: `http://localhost:5432` (database port, for client connections)
- Redis: `http://localhost:6379` (Redis port)
- API Gateway: `http://localhost:8080/health`
- Admin API: `http://localhost:8098/health`
- Auth Service: `http://localhost:8081/health`
- Analytics Service: `http://localhost:8097/health`
- Recommendation Service: `http://localhost:8093/health`
- Moderation Service: `http://localhost:8094/health`

NATS monitoring is available at `http://localhost:8222` when the optional `nats` profile is enabled.

## Running tests locally

```bash
make test
```

This includes the verified Go and Python test suites in the repo.

## Local builds

```bash
make build
```

This builds the Go modules and also performs Docker image builds for the supported default applications.

## Stop the stack

Stop without deleting persistent data:

```bash
make infra-down
```

Stop and remove persistent volumes (destructive):

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml down -v
```

> Warning: `down -v` deletes the local PostgreSQL and Redis data volumes. Only use it when you intentionally want to reset local development data.

## Reset development data

Use the destructive reset only when you want a clean slate:

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml down -v
```

If you want to keep your local data, run the non-destructive stop above instead.

## Troubleshooting

Common startup issues:

- If Docker reports a port conflict, change the host ports in `.env` before starting the stack.
- If a service fails because a database is missing, re-run `make migrate` after the PostgreSQL container is healthy.
- If a service exits early, inspect its logs with `make infra-logs` and check the `.env` file entries for the relevant URLs and secrets.
- If Compose fails to parse, run `make infra-config` to surface the exact config problem.
- If you are starting the optional NATS profile, include `--profile nats` explicitly.

## Service ownership and constraints

The local stack reflects real, verified service ownership and startup behavior instead of assuming everything in the repository is runnable.

- PostgreSQL is the shared local datastore for the default services.
- Redis is required by the Auth Service.
- NATS is optional and not required by the default stack.
- Services outside the default profile are not started automatically because they were not part of the verified default local workflow.

## Useful commands

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml ps
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml logs -f postgres redis
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml down
```
