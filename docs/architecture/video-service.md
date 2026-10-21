# Video Service Architecture

## Boundary

```text
content-service  (edvance_content)
      ↓  owns WHAT the video is and WHO may see it:
         title, description, tags, category, language, visibility,
         moderation, publication, discovery, access decisions
video-service    (edvance_video)
      ↓  owns the MEDIA and its pipeline:
         media_assets, uploads, processing state, renditions,
         thumbnails, captions, playback facts
media-engine (future Go + FFmpeg service)
      ↓  owns PROCESSING the actual media
object storage (future)
      ↓  owns BYTES
CDN (future)
      ↓  owns DELIVERY
```

There is exactly one `videos` table, in content-service. video-service
has no video-metadata table and does not know titles, visibility or
moderation. The two services are linked by the opaque `media_asset_id`
stored on the content row.

## Data ownership

```text
video-service   ❌ SELECT from edvance_content / edvance_creator / ...
video-service   ✅ stores owner_id as a plain UUID (JWT sub)
content-service ✅ stores media_asset_id as a plain UUID
```

No cross-service foreign keys and no cross-service JOINs. Each service
owns its own database and migrations.

## Identity flow

```text
Client
  ↓  Authorization: Bearer <JWT from auth-service>
API Gateway  (strips /api/v1/videos, no business logic)
  ↓
Video Service  (validates JWT locally: signature, expiry,
                issuer, audience, HS256-only)
  ↓  sub → owner_id
PostgreSQL (edvance_video)
```

Upload ownership comes only from the JWT `sub`; client-supplied user IDs
are never trusted.

## Upload and processing flow

```text
Client (creator owner)
  ↓  POST /initiate {filename,mimeType,size,
                    educationalConfirmed,rightsConfirmed}
  ↓                                     created → uploading
  ↓  PUT bytes directly to presigned storage URL
  ↓  POST /{mediaAssetID}/complete       uploading → uploaded
Worker
  ↓  claim job (FOR UPDATE SKIP LOCKED)  uploaded → processing
  ↓  dispatch to engine
Engine
  ↓  POST /internal/v1/videos/processing/callback
  ↓                                     processing → ready | failed
  ↓  POST /{mediaAssetID}/retry-processing  failed → processing
Player
  ↓  GET /internal/v1/videos/{mediaAssetID}/playback  (content-service only)
```

Video uploads are gated on two attestations — that the upload is
educational content and that the uploader holds the rights. The
timestamps are persisted on the asset. Audio and image uploads are not
gated.

The internal routes are separate from user routes, are not proxied by
the gateway, and require the shared service token. content-service is
the playback authority: it checks ownership/visibility first, then asks
video-service for player-facing facts. video-service never decides who
may watch.

## Content-service integration

Progress is announced best-effort to content-service at `uploaded`,
`processing`, `ready` and `failed`:

```text
POST {CONTENT_SERVICE_URL}/internal/media-assets/{mediaAssetID}/status
Authorization: Bearer <CONTENT_SERVICE_INTERNAL_TOKEN>

{status, videoId?, durationSeconds?, thumbnailUrl?,
 errorCode?, errorMessage?}
```

Notifications are idempotent and non-fatal: an outage or a `404` (row
not found) never rolls back the pipeline. The callback is the only
writer of media-derived fields on the content row.

## State machine

```text
created → uploading → uploaded → processing → ready
                       processing → failed
                       failed → processing (retry processing)
                       * → deleted
```

Transitions are explicit (`CanTransition`) and enforced again by
conditional SQL (`WHERE status = $expected`), so concurrent callbacks or
a racing delete resolve into conflicts, never skipped steps.

## Deliberate limitations (v1)

- No NATS/Kafka; notifications are direct HTTP calls.
- No workers running FFmpeg in-repo; the engine is an external boundary
  with `cmd/mockengine` as the local stand-in.
- Ownership trusts JWT identity until gRPC authorizers land.
- One primary thumbnail and one default caption are promoted; alternate
  tracks are stored but not exposed as a choice yet.
- No CDN invalidation on delete; cleanup deletes tracked objects only.
