# NATS Messaging in Edvance

## Why NATS

Edvance uses **NATS with JetStream** as its internal, durable event bus. It is
used **only where asynchronous communication provides a real benefit**:

- fanning out a business event to multiple independent consumers (for
  example, `course.published` updates search, analytics, and notifications),
- long-running or retriable background work (video transcoding, email
  delivery retries), and
- durable delivery that must survive a consumer restart without losing
  events.

Synchronous HTTP remains the correct choice for login/auth, fetching a
video or course, checking permissions or enrollment, and any operation
that must return an immediate validation result. NATS is **not** a
replacement for PostgreSQL, Redis, R2, or the API gateway, and it never
carries media bytes.

NATS is an **internal infrastructure dependency**. It is not exposed to
students or creators, and no public gateway route publishes to it.

## Event model

All events share one wire envelope (`packages/events`):

| Field | Meaning |
|-------|---------|
| `id` | UUID v4, unique per occurrence; used for idempotent dedup |
| `type` | `domain.event`, e.g. `video.processing.completed` |
| `version` | Payload schema version (currently `1`) |
| `occurred_at` | RFC3339 timestamp of the business fact |
| `source` | Emitting service, e.g. `video-service` |
| `correlation_id` | Optional workflow link (payment -> enrollment) |
| `data` | JSON payload — opaque IDs and minimal fields only |

### Naming convention

Subjects use `edvance.<domain>.<event>`, e.g.
`edvance.video.processing.completed`. A single JetStream stream catches
`edvance.>` and consumers subscribe narrowly.

### Event contracts (implemented)

Only event families whose features exist in the repo are defined.

| Subject | Publisher | Consumers |
|---------|-----------|-----------|
| `edvance.video.uploaded` | video-service | (future) |
| `edvance.video.processing.requested` | video-service | notification-service |
| `edvance.video.processing.completed` | video-service | notification-service, analytics-service |
| `edvance.video.processing.failed` | video-service | notification-service |
| `edvance.video.published` | content-service | notification-service, search-service, analytics-service |
| `edvance.video.unpublished` | content-service | search-service |
| `edvance.content.published` | content-service | notification-service |
| `edvance.content.unpublished` | content-service | search-service |
| `edvance.course.published` | course-service* | notification-service, search-service, analytics-service |
| `edvance.course.updated` | course-service* | notification-service, search-service |
| `edvance.course.unpublished` | course-service* | search-service |
| `edvance.course.deleted` | course-service* | search-service |
| `edvance.enrollment.created` | learning-service* | notification-service, analytics-service |
| `edvance.learning.lesson.completed` | learning-service* | analytics-service |
| `edvance.payment.succeeded` | payment-service | notification-service, analytics-service |
| `edvance.payment.failed` | payment-service | notification-service |
| `edvance.payment.refunded` | payment-service | notification-service |
| `edvance.live.session.created` | live-streaming-service* | analytics-service |
| `edvance.live.session.started` | live-streaming-service* | analytics-service |
| `edvance.live.session.ended` | live-streaming-service* | analytics-service |

\* Marked with an asterisk where the publisher integration is planned
but not yet wired in this implementation; the event contract and subject
are reserved.

### Payload rules

- Opaque identifiers only (video ID, payment ID, course ID).
- Never passwords, access tokens, R2 credentials, signed URLs, or
  sensitive personal data.
- Minimal fields: enough for the consumer to act, nothing more.

## Streams and durable consumers

One logical JetStream stream (`edvance.events`, subject `edvance.>`)
holds business events with **Limits** retention, 72h max age, 1 replica
(local dev). A separate work-queue stream (`edvance.events.outbox`,
subject `edvance.outbox.>`, 24h retention) backs the transactional
outbox.

Streams are created once by `packages/events.EnsureStream` at startup.
An existing stream is **never deleted or overwritten**: if it exists but
lacks a requested subject, `EnsureStream` returns an error so an
operator can reconcile intentionally.

Durable consumer names are stable per service (`notification-service`,
`analytics-service`, `search-service`). Consumers use manual
acknowledgment, an AckWait deadline, and a bounded `MaxDeliveries`
(redelivery ladder). Messages are acknowledged **only after** the
business effect succeeds (or is recorded as terminal), so consumers
recover from failure without losing events.

## Reliability

### Transactional outbox

`content-service` writes publish/unpublish events to an `event_outbox`
table **in the same PostgreSQL transaction** as the business row. A
background `OutboxPublisher` loop claims pending rows (`FOR UPDATE
SKIP LOCKED`), publishes them to JetStream, and marks each row
published only after a server acknowledgment. Attempts and terminal
failures are tracked. This guarantees an event is never lost between a
database commit and JetStream publication.

### Idempotency

Every consumer is idempotent by event ID:
- `notification-service` dedupes notifications by `event_id`.
- `analytics-service` inserts with `ON CONFLICT (event_id) DO NOTHING`.
- `search-service` upserts by `(source_type, source_id)`.

Redelivery therefore never duplicates business effects. Delivery is
**at-least-once**, not exactly-once.

### Failure handling

- Handlers return `nil` -> ack.
- Handlers return a non-terminal error -> nack, bounded redelivery.
- Handlers return `events.Terminal(err)` -> ack and log to the
  dead-letter/failed path for operator replay.
- Malformed messages are acked and logged (discarded), never redelivered
  forever.

## Configuration

| Variable | Default | Meaning |
|----------|---------|---------|
| `NATS_URL` | `nats://localhost:4222` | Broker endpoint |
| `NATS_USER` / `NATS_PASSWORD` | (empty) | Plaintext credentials; empty = none |
| `NATS_JETSTREAM` | `true` | Enable JetStream |
| `NATS_STREAM` | `edvance.events` | Stream name |
| `NATS_SUBJECT` | `edvance.>` | Publish/consume subject |
| `NATS_DURABLE` | per-service | Durable consumer name |
| `NATS_PUBLISH_TIMEOUT` | `10s` | Sync publish ack deadline |
| `NATS_ACK_TIMEOUT` | `30s` | Consumer ack deadline |
| `NATS_MAX_DELIVERIES` | `5` | Redelivery limit |
| `NATS_MAX_AGE` | `72h` | Stream retention age |
| `NATS_MAX_BYTES` | `0` (unlimited) | Stream byte cap |
| `NATS_MAX_MSGS` | `0` (unlimited) | Stream message cap |
| `NATS_REPLICAS` | `1` | Stream replicas |
| `NATS_TLS` | `0` | `1` enables TLS |

Credentials must never be committed; use `.env` (gitignored) or the
runtime environment.

## Local setup

NATS is part of the default Compose stack:

```bash
# Start infrastructure including NATS + JetStream
make infra-up

# Verify NATS health
docker compose --env-file .env -f infrastructure/docker/docker-compose.yml ps nats
# expect: edvance-nats Healthy
```

The `nats` service runs `nats:2.10-alpine` with `-js` (JetStream) and
persistent storage (`nats_data` volume). Its monitoring endpoint is on
port 8222.

### Inspecting streams and consumers

```bash
# List streams
docker exec edvance-nats nats stream ls

# Stream info
docker exec edvance-nats nats stream info edvance.events

# List consumers
docker exec edvance-nats nats consumer ls edvance.events

# Watch pending messages for a consumer
docker exec edvance-nats nats consumer info edvance.events notification-service
```

Or hit the monitoring endpoint:

```bash
curl -s http://localhost:8222/streaming | python3 -m json.tool
```

## Event flow diagram

```text
                      +---------------------+
                      |   content-service   |
                      |  (publish video)    |
                      +---------+-----------+
                                |
                                | 1. DB tx: video row + outbox row
                                v
                      +---------------------+
                      |   OutboxPublisher   |  (background loop)
                      +---------+-----------+
                                |
                                | 2. JetStream publish
                                v
                      +---------------------+
                      |      NATS           |  JetStream stream: edvance.events
                      |  subject:           |  durable consumers:
                      |  edvance.video.*    +---> notification-service
                      |                     +---> analytics-service
                      |                     +---> search-service
                      +---------------------+

                      +---------------------+
                      |   video-service     |  (media pipeline)
                      |  (callback ready)   |
                      +---------+-----------+
                                | 3. JetStream publish
                                v
                      edvance.video.processing.completed
```

Publisher -> outbox/db -> JetStream -> durable consumers -> business
effect (notification, search index, analytics). Each consumer acks only
after its effect succeeds; failures nack for redelivery (bounded) or go
terminal (logged for replay).

## Operational commands

```bash
# Start everything (includes NATS)
make infra-up

# Run the Go test suites (includes packages/events and service tests)
make test

# Build all Go modules
make build

# Validate Compose (includes the nats service)
make infra-config
```

## What is NOT over NATS (deliberate)

- Authentication / login (synchronous HTTP, JWT).
- Media bytes or video uploads (presigned R2 PUT).
- Synchronous reads (fetch video/course/creator).
- Cross-service database access — each service owns its DB and learns
  about other domains only through events.
