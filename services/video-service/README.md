# Video Service

Owns the **media asset pipeline** for Edvance: direct-to-storage uploads,
asynchronous transcode orchestration, rendition/thumbnail/caption
metadata, and playback URLs. FFmpeg never runs here; the external media
engine owns byte-level processing.

Port: `8085` · Database: `edvance_video` · Module:
`github.com/Anshul563/edvance-project/services/video-service`

## Responsibilities

- media asset lifecycle (`media_assets`) and ownership
- presigned direct upload (bytes never traverse this service)
- processing job queue (`processing_jobs`) with a `FOR UPDATE SKIP
  LOCKED` claim and attempts-based exponential backoff
- engine callback ingestion (variants, thumbnails, captions, metadata)
- playback URL derivation from the object store
- cleanup orchestration for deleted assets

## Non-responsibilities

Users, creators, content titles/descriptions, courses, lessons,
comments, payments, recommendations, search, FFmpeg execution. Those
belong to other services or the external media engine.

## Environment

Copy `.env.example` to `.env`. `JWT_ACCESS_SECRET` must match
auth-service (this service only validates). `INTERNAL_SERVICE_TOKEN`
protects `/internal/v1/*`; it is never a user JWT and never proxied by
the gateway. `S3_*` describe any S3-compatible endpoint (R2, S3, MinIO):
when `S3_PUBLIC_URL` is set, playback URLs are stable public links,
otherwise they are presigned. `MEDIA_ENGINE_URL` points at the engine
that owns FFmpeg; the repository ships `cmd/mockengine` as a local
development stand-in.

## Endpoints

Authenticated (JWT, ownership enforced):

```text
POST   /initiate                     {filename, mimeType, size} -> 201 {uploadUrl, asset}
POST   /{mediaAssetID}/complete      (no body)                   -> 200 asset (queues transcode)
GET    /{mediaAssetID}                                            -> 200 asset + renditions
GET    /?status=&limit=&offset=                                  -> 200 {assets}
DELETE /{mediaAssetID}                                           -> 204 (soft delete + cleanup job)
```

Internal (media engine only, `X-Internal-Key: <INTERNAL_SERVICE_TOKEN>`,
never proxied by the gateway):

```text
POST /internal/v1/videos/processing/callback   -> 202 (idempotent)
```

Health:

```text
GET /health
GET /ready
```

Through the gateway, prefix user routes with `/api/v1`: the gateway
strips `/api/v1/videos` and forwards the rest to this service.

## Upload flow

1. `POST /initiate` creates an asset (`created → uploading`) and returns
   a presigned PUT bound to the validated MIME type. Storage key is
   server-derived (`videos/{owner}/{asset}/original.{ext}`), never the
   client filename; path-traversal filenames are rejected.
2. The client PUTs bytes directly to that URL with the matching
   `Content-Type`. Bytes never pass through this service.
3. `POST /{id}/complete` verifies the object with a HEAD (a client lying
   about upload completion gets `ErrUploadNotPresent`, not a status
   change), moves the asset to `uploaded`, and enqueues one `transcode`
   job holding `sourceStorageKey`, `outputPrefix` and
   `maxDurationSeconds`.
4. The worker claims the job (`FOR UPDATE SKIP LOCKED`,
   highest-priority/oldest first), moves the asset to `processing`, and
   dispatches it to the engine. Dispatch failures retry with backoff;
   an exhausted budget fails the job and the asset.
5. The engine posts the callback. Success upserts renditions keyed on
   storage key (idempotent), promotes one primary thumbnail and one
   default caption, persists engine metadata, marks the asset `ready`
   and the job `completed`. Failure requeues while retriable or fails
   the asset when the budget or retriability is exhausted.

Delete marks the asset `deleted` (instantly stops playback; every read
path checks status) and enqueues a `cleanup` job that deletes every
tracked object. No byte is deleted synchronously.

## State machine

```text
created → uploading → uploaded → processing → ready
                       processing → failed
                       failed → processing (retry retranscodes)
                       * → deleted
```

Transitions are explicit (`CanTransition`) and enforced again by
conditional SQL (`WHERE status = $expected`), so concurrent callbacks or
a racing delete resolve into conflicts, never skipped steps.

## Security notes

- Ownership comes only from the JWT `sub`; client-supplied user IDs are
  never trusted.
- Engine metadata is the *only* writer of duration/dimensions/codec/
  renditions. Client requests can neither inject it nor overwrite it.
- The callback is idempotent: completed jobs are acked without writes,
  and child rows key on unique storage keys.
- `INTERNAL_SERVICE_TOKEN` is constant-time compared and never logged.

## Local development

```bash
psql "$DATABASE_URL" -f migrations/001_create_video_tables.sql
go run ./cmd/s3stub &                                        # S3-compatible stand-in (or real MinIO/R2/S3)
go run ./cmd/mockengine &                                     # engine stand-in
go run ./cmd/server
```

`cmd/s3stub` is an in-memory S3-compatible store that satisfies the
service's PUT/HEAD/DELETE calls, so local e2e needs no Docker or MinIO.
`cmd/mockengine` accepts jobs and posts a fully-formed success callback
after a short delay, exercising the whole orchestration path without
FFmpeg. It honors `payload["mock:fail"]`, `payload["mock:retriable"]`
and `payload["mock:delay"]` for failure-path testing. Run mockengine
with the same `MOCKENGINE_*`-style env used by the service:

```bash
MEDIA_ENGINE_TOKEN=dev-media-engine-token \
INTERNAL_SERVICE_TOKEN=dev-video-internal-token-change-me \
go run ./cmd/mockengine
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_video?... \
  go test -tags integration ./...
```