# Media Service

Owns **media processing orchestration**: job records, lifecycle,
idempotency, retries, and the boundary to the external Go + FFmpeg
media engine.

Port: `8091` · Database: `edvance_media` · Module:
`github.com/Anshul563/edvance-project/services/media-service`

## Responsibilities

- media processing jobs (`video_transcode`, `thumbnail_generate`, `video_probe`)
- job lifecycle (`queued → running → completed/failed/cancelled`)
- idempotent job creation (DB unique constraint, not memory)
- retries as NEW jobs (originals preserved for debugging)
- engine dispatch / polling / cancellation via `internal/engine`
- result mapping (outputs on success, normalized errors on failure)

## Non-responsibilities

FFmpeg, transcoding, probing, thumbnails, HLS/DASH, R2/S3, CDN, uploads,
courses, lessons, comments, likes, payments, analytics, Kafka/NATS,
workers (a future worker can dispatch `queued` jobs untouched).

## Environment

```env
APP_ENV=development
MEDIA_SERVICE_PORT=8091
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_media?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
MEDIA_ENGINE_URL=http://localhost:9000
MEDIA_ENGINE_INTERNAL_TOKEN=   # known ONLY to this service, never to clients
```

## Endpoints (all JWT-authenticated, ownership enforced)

```text
POST /jobs                          {videoId, jobType, sourceObjectKey, idempotencyKey?}
GET  /jobs/:jobID
POST /jobs/:jobID/refresh           poll engine, reconcile local state
POST /jobs/:jobID/cancel            engine first, then local (terminal: 409)
POST /jobs/:jobID/retry             failed only -> NEW job, attempt+1
GET  /videos/:videoID/jobs?page=&limit=&jobType=&status=
```

Health: `GET /health` (alive), `GET /ready` (PostgreSQL only — the
engine is deliberately NOT a readiness dependency so orchestration
state stays readable during engine outages).

Through the gateway prefix everything with `/api/v1/media` (stripped):
`POST /api/v1/media/jobs`, …

## Authentication & authorization

JWTs validated locally (HS256-only, issuer/audience/expiry); `sub` is
the identity. `VideoAuthorization` (`CanManageVideo`) is an isolated
seam: v1 trusts the JWT identity, later replaced by internal gRPC to
video-service. Denials fail closed.

## Job lifecycle

```text
queued → running → completed
               running → failed
        queued/running → cancelled
```

Terminal states never transition; retries create new jobs with
`attempt_count + 1` and derived keys (`…-v1` → `…-v2`).

## Idempotency

Repeat submission of an `idempotencyKey` returns the original job
(200-path included in `201` responses; no duplicate rows, no second
engine dispatch — verified by test). Keys are scoped per video+type: a
key reused across videos is rejected (`409`) instead of leaking
another video's job.

## Engine contract (`internal/engine`)

Assumed future engine API: `POST /v1/jobs`, `GET /v1/jobs/:id`,
`POST /v1/jobs/:id/cancel`. Short control-plane timeouts (10s) — the
service never holds a request open while FFmpeg runs. Unknown engine
statuses never map into terminal states. Engine failures preserve the
queued job and report `502` with its id (`{"error":…, "jobId":…}`).

## Local development

No engine runs locally yet; point `MEDIA_ENGINE_URL` at a mock or
leave jobs `queued` (they survive restarts for a future worker).

```bash
psql "$DATABASE_URL" -f migrations/001_create_media_jobs.sql
go run ./cmd/server
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_media?... \
  go test -tags integration ./...
```

Unit tests use fake store/engine/authorizer; integration uses real
PostgreSQL plus an in-test HTTP mock engine.
