# Video Service

Owns the **media representation** of Edvance content: video records, their
processing lifecycle, source/playback metadata, and playback rules.

Port: `8085` · Database: `edvance_video` · Module:
`github.com/Anshul563/edvance-project/services/video-service`

## Responsibilities

- video records and the video/content relationship (1:1)
- processing status lifecycle
- source media metadata (`source_object_key`)
- processed media metadata (duration, dimensions, thumbnail, manifest)
- public playback rules
- soft deletion

## Non-responsibilities

Creator profiles, user accounts, content titles/descriptions, courses,
lessons, comments, likes, payments, recommendations, search, FFmpeg
execution, object storage, CDN. Those belong to other services or the
future media engine.

## Environment

```env
APP_ENV=development
VIDEO_SERVICE_PORT=8085
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_video?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
INTERNAL_API_KEY=    # shared secret for media-engine callbacks
```

## Endpoints

Public (no auth):

```text
GET /videos/:videoID
GET /videos/content/:contentID
GET /videos/creator/:creatorID?page=1&limit=20&status=&includeDeleted=
```

Authenticated (JWT, ownership enforced):

```text
POST   /videos                    {contentId, creatorId} -> 201 pending
PATCH  /videos/:videoID/source    {sourceObjectKey}      -> pending→uploading
DELETE /videos/:videoID                                  -> soft delete
```

Internal (media engine only, `X-Internal-Key` header, never proxied by
the gateway):

```text
POST /internal/videos/:videoID/processing
```

Health:

```text
GET /health
GET /ready
```

Through the gateway, prefix everything with `/api/v1` (the gateway
strips it): `POST /api/v1/videos`, `GET /api/v1/videos/:id`, …

## Authentication

JWTs are validated locally (signature, expiry, issuer, audience,
HS256-only); `sub` is the user identity. auth-service is never called
per request. Ownership (`CreatorAuthorization` / `ContentAuthorization`)
is an isolated seam: v1 trusts the JWT identity, later replaced by
internal gRPC without touching handlers.

## Ownership

`user_id`, `creator_id`, `content_id`, `video_id` are distinct
identities. The service never assumes `user_id == creator_id`.
`content_id`/`creator_id` are plain-UUID cross-service references —
no cross-database foreign keys, no cross-database queries.

## Lifecycle

```text
pending → uploading → processing → ready
processing → failed
* → deleted (terminal, soft delete)
```

Transitions are validated centrally (`CanTransition`) and enforced again
by conditional SQL (`UPDATE ... WHERE status = $expected`), so races
resolve into conflicts, never skipped steps. `ready` requires a
playback manifest.

## Playback rules

Only `ready` + non-null manifest is playable. Public responses never
contain `sourceObjectKey` or `processingError`; the manifest appears
only when playable. Owners (optional auth) see the full record. Deleted
rows are 404 everywhere except owner `?includeDeleted=true` listings.

## Local development

```bash
psql "$DATABASE_URL" -f migrations/001_create_videos.sql
go run ./cmd/server
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_video?... \
  go test -tags integration ./...
```
