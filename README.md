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

- Course, Learning, and Commerce Services (`--profile purchase`).
- Payment Service (`--profile payment`), requiring Razorpay test credentials in `.env`.

Unsupported by default:

- User, Creator, Search, Notification, Media, Video, and Live Streaming services are present in the repo, but not included in the default Compose profile because this stack is intentionally limited to verified startup paths.
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
make infra-up
```

## Start the default application stack

```bash
make app-up
```

These services use Compose DNS hostnames instead of `localhost` so the containers can resolve one another correctly.

## Apply migrations

The local database initialization script creates the required databases under PostgreSQL, but it does not apply service migrations automatically. Apply the migrations explicitly:

```bash
make migrate
```

This runs the SQL migrations for the included Go services and the Alembic migrations for the included Python services.

## Run the purchase and learning workflow

The purchase stack is optional and intentionally separate from the default stack. First set `LEARNING_SERVICE_INTERNAL_TOKEN` and `COMMERCE_SERVICE_INTERNAL_TOKEN` in `.env` to local development values. To exercise real provider checkout callbacks, configure Razorpay test-mode credentials and a webhook endpoint at `http://localhost:8090/webhooks/razorpay`.

```bash
make migrate
make purchase-up
```

Course Service has no global public catalog list; the available discovery contract is `GET /api/v1/courses/creator/{creatorID}` plus `GET /api/v1/courses/{courseID}`. The learner creates an order with `POST /api/v1/commerce/orders`. Send an `Idempotency-Key` header when creating orders; identical retries return the original order, and a reused key with a different course/coupon payload is rejected. Start payment with `POST /api/v1/payments/` using the returned `commerceOrderId`; verify the provider callback with `POST /api/v1/payments/verify`. Payment status is read with `GET /api/v1/payments/{paymentID}` or `GET /api/v1/payments/order/{commerceOrderID}`. Refunds use `POST /api/v1/payments/{paymentID}/refund` with a required `Idempotency-Key`; repeating the same key will not call the provider twice. Purchase history is `GET /api/v1/commerce/purchases`; enrolled learning access is `GET /api/v1/learning/courses/{courseID}/enrollment` and learning progress routes.

For free published courses, use `POST /api/v1/learning/courses/{courseID}/enroll`; this direct free path does not create a payment or paid purchase.

Only public gateway routes are exposed. Internal Commerce and Learning enrollment/refund callbacks require `X-Internal-Key` and are not mounted by the gateway. A confirmed refund updates Commerce order/purchase state; it does not automatically revoke an existing Learning enrollment. Unknown provider outcomes remain reserved and are not retried as a second provider operation; replay the same key to inspect the pending state and reconcile it through the provider webhook.

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

- PostgreSQL: `localhost:5432` (database TCP endpoint)
- Redis: `localhost:6379` (Redis TCP endpoint)
- API Gateway: `http://localhost:8080/health`
- Admin API: `http://localhost:8098/health`
- Auth Service: `http://localhost:8081/health`
- Analytics Service: `http://localhost:8097/health`
- Recommendation Service: `http://localhost:8093/health`
- Moderation Service: `http://localhost:8094/health`

Purchase profile health endpoints are Course `http://localhost:8086/health`, Learning `http://localhost:8087/health`, Commerce `http://localhost:8089/health`, and Payment `http://localhost:8090/health`.

## Running tests locally

```bash
make test
```

This includes the verified Go and Python test suites in the repo.

## Local builds

```bash
make build
```

This builds the Go modules and Docker images for the default stack and purchase services. Payment Service requires Razorpay test credentials to start, but credentials are not needed to build it.

## Stop the stack

Stop without deleting persistent data:

```bash
make infra-down
```

Stop and remove persistent volumes (destructive):

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml down -v
```

> Warning: `down -v` deletes the local PostgreSQL data volume. Redis is intentionally ephemeral and has no persistent volume.

## Reset development data

Use the destructive reset only when you want a clean slate:

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml down -v
```

If you want to keep your local data, run the non-destructive stop above instead.

## Troubleshooting

Common startup issues:

- If PostgreSQL or Redis reports a port conflict, change `POSTGRES_PORT` or `REDIS_PORT` in `.env`. Application host ports are declared in `infrastructure/docker/docker-compose.yml` and must be changed there.
- If a service fails because a database is missing, re-run `make migrate` after the PostgreSQL container is healthy.
- If a service exits early, inspect its logs with `make infra-logs` and check the `.env` file entries for the relevant URLs and secrets.
- If Compose fails to parse, run `make infra-config` to surface the exact config problem.
- If Payment Service exits at startup, verify `RAZORPAY_KEY_ID`, `RAZORPAY_KEY_SECRET`, and `RAZORPAY_WEBHOOK_SECRET` are set to test-mode values; empty defaults deliberately fail closed.
- If Commerce reports a 401 while provisioning Learning, ensure both services use the same `LEARNING_SERVICE_INTERNAL_TOKEN` value.

## Service ownership and constraints

The local stack reflects real, verified service ownership and startup behavior instead of assuming everything in the repository is runnable.

- PostgreSQL is the shared local datastore for the default services.
- Redis is required by the Auth Service.
- No current purchase-path service uses NATS, so NATS is not provisioned by this Compose stack.
- Services outside the default profile are not started automatically because they were not part of the verified default local workflow.

## Useful commands

```bash
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml ps
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml logs -f postgres redis
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml down
```
