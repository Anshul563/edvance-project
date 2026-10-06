# Media Service Architecture

## Boundary

```text
video-service
      ↓  owns video identity, lifecycle, playback metadata
media-service
      ↓  owns jobs, orchestration, engine boundary
media-engine (future external Go service)
      ↓  owns FFmpeg: probe, transcode, thumbnails, HLS/DASH
object storage (future)
      ↓  owns BYTES
CDN (future)
      ↓  owns DELIVERY
```

## Why FFmpeg does NOT belong in media-service

- **Independent scaling**: FFmpeg is CPU-bound and bursty; orchestration
  is I/O-bound and steady. One deployment cannot size both.
- **Independent deployability**: engine upgrades (codecs, FFmpeg
  versions) must not restart job state or API serving.
- **Reusability**: the engine is intended to serve Edvance, LMS
  platforms, and YouTube-like apps — not one product's monolith.
- **Blast radius**: a crashing transcode must kill a worker, not the
  service holding every job's state.

media-service therefore holds **state + policy** (lifecycle, idempotency,
retries, mapping) and the engine holds **execution**. The seam is the
`engine.Engine` interface: HTTP today, gRPC later, fakes in tests.

## Identity flow

```text
Client
  ↓  Authorization: Bearer <JWT from auth-service>
API Gateway  (strips /api/v1/media, no business logic)
  ↓
Media Service  (validates JWT locally; sub → user_id)
  ↓  VideoAuthorization.CanManageVideo
     (v1: trust JWT identity · later: gRPC to video-service)
PostgreSQL (edvance_media) + Media Engine (dispatch/poll/cancel)
```

## Orchestration flow (current, synchronous seams)

```text
Client
  ↓  POST /jobs {videoId, type, source, idempotencyKey?}
Media Service: authorize → idempotency check → INSERT queued
  ↓  dispatch to engine (10s control timeout)
Engine accepts → store engine_job_id → running → return job
Engine down    → keep queued job → 502 + jobId (never lost)
  ↓  processing happens separately (async, never blocks HTTP)
Client polls (or future worker/event):
  ↓  POST /jobs/:id/refresh → GET engine status → map → persist
Engine completed → store manifest/thumbnail/duration/dims
Engine failed    → store normalized code/message (capped, no traces)
```

## Future event-driven processing

Planned events (not implemented — no broker yet):

```text
video.created
video.uploaded
video.processing.started
video.processing.completed
video.processing.failed
```

Ready for them: `UpdateStatus`-style conditional writes, terminal
states that never move, retries-as-new-jobs, and idempotency keys make
duplicate/redelivered events converge instead of corrupting state. A
background worker can later `SELECT ... WHERE status='queued'` and call
the existing `dispatch` path unchanged.

## Video-service callback (future, NOT implemented)

When the pipeline finishes, video-service needs the playback metadata.
That integration is deliberately absent: no callbacks, no shared tables.
The future shape is either a media-service→video-service internal call
(using video-service's existing internal processing route) or an event
the video-service consumes. Both fit without schema changes here.

## Deliberate limitations (v1)

- No workers, NATS/Kafka, R2/S3, CDN, uploads, HLS/DASH.
- Ownership trusts JWT identity until gRPC authorizers land.
- `ready`-without-manifest rejected; no reprocessing workflow.
- Engine outages surface as `502`; jobs wait `queued` for retry/refresh.
