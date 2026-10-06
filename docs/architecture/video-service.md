# Video Service Architecture

## Boundary

```text
content-service
      ↓  owns WHAT the content is (title, description, lessons, order)
video-service
      ↓  owns the MEDIA REPRESENTATION of that content
         (record, processing state, source/playback metadata)
media-engine (future Go + FFmpeg service)
      ↓  owns PROCESSING the actual media
object storage (future)
      ↓  owns BYTES
CDN (future)
      ↓  owns DELIVERY
```

The important distinction:

- **content-service** — what the content is.
- **video-service** — the media representation of that content.
- **media-engine** — processing the actual media (will own FFmpeg).

## Data ownership

Each service owns its database. video-service stores only its own
`videos` rows. `content_id` and `creator_id` are opaque UUID references:

```text
video-service   ❌ SELECT from edvance_content / edvance_creator / ...
video-service   ✅ stores content_id / creator_id as plain UUIDs
```

No cross-service foreign keys. No cross-service JOINs.

## Identity flow

```text
Client
  ↓  Authorization: Bearer <JWT from auth-service>
API Gateway  (strips /api/v1/videos, no business logic)
  ↓
Video Service  (validates JWT locally: signature, expiry,
                issuer, audience, HS256-only)
  ↓  sub → user_id
CreatorAuthorization / ContentAuthorization
  (v1: trust JWT identity · later: internal gRPC to
   creator-service / content-service)
  ↓
PostgreSQL (edvance_video)
```

## Processing flow (current, synchronous seams)

```text
Client (creator owner)
  ↓  POST /videos {contentId, creatorId}          → pending
  ↓  PATCH /videos/:id/source {sourceObjectKey}  → uploading
Media engine (future; today: internal route + shared key)
  ↓  POST /internal/videos/:id/processing {processing} → processing
  ↓  POST /internal/videos/:id/processing {ready, …}    → ready
Client / player
  ↓  GET /videos/:id  (manifest only when playable)
```

The internal route is intentionally separate from public routes and is
not proxied by the gateway. Its shared-key guard is a placeholder for
mTLS / service mesh.

## Future event-driven processing

Planned events (not implemented — no broker yet):

```text
video.created
video.uploaded
video.processing.started
video.processing.completed
video.processing.failed
```

The service is already compatible: state changes funnel through
`UpdateProcessingState`, and all writes are conditional on the expected
previous status, so duplicate/redelivered events resolve into
conflicts instead of corrupt state.

## Deliberate limitations (v1)

- One video per content (UNIQUE content_id); no multi-track assets.
- Handles, courses, lessons, comments, likes, payments, search,
  recommendations, analytics: other (mostly future) services.
- No file upload, transcoding, HLS/DASH, R2/S3, or CDN integration.
- Ownership checks trust the JWT identity until gRPC authorizers land.
- `ready` without a manifest is rejected; reprocessing a failed video
  needs an explicit future workflow (`failed` is movable only to
  `deleted` today).
