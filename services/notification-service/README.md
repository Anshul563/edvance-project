# Notification Service

Centralized **delivery infrastructure** for Edvance user notifications:
in-app storage plus email sending, preferences, templates, retries, and
idempotent intake. It decides *how* to deliver; originating services
decide *when* a business event occurred.

Port: `8092` · Database: `edvance_notification` · Module:
`github.com/Anshul563/edvance-project/services/notification-service`

## Responsibilities

- notifications (one row per user-visible event, any channel mix)
- delivery records (per-channel send state, independent retries)
- preferences (channel/category toggles, mandatory security)
- templates (DB-backed subject/title/body rendering)
- console + SMTP email providers behind one interface
- internal event intake (`POST /internal/v1/notifications/events`)
- retry schedule (1m, 5m, 15m, 1h; max 4 attempts)

## Non-responsibilities

Business-domain state and decisions, Kafka/NATS, WebSockets, push/SMS
providers, FCM, template admin UI, recommendations. No other service's
database is ever touched.

## Environment

```env
APP_ENV=development
NOTIFICATION_SERVICE_PORT=8092
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_notification?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
EMAIL_PROVIDER=console   # console (dev only) | smtp
SMTP_HOST=
SMTP_PORT=587
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_FROM=
NOTIFICATION_INTERNAL_TOKEN=   # trusted services only, never frontend

# Delivery retry worker: pollers, poll period, and max send attempts.
NOTIFICATION_WORKER_COUNT=4
NOTIFICATION_RETRY_INTERVAL=1m
NOTIFICATION_MAX_ATTEMPTS=4
```

## Endpoints (all JWT-authed, strictly per-user)

```text
GET    /notifications?page=&limit=&unread=
GET    /notifications/unread-count
POST   /notifications/read-all
POST   /notifications/:notificationID/read
DELETE /notifications/:notificationID
GET    /notifications/preferences
PATCH  /notifications/preferences
GET    /health  |  GET /ready
```

Internal (`Authorization: Bearer <token>`, never proxied by gateway):

```text
POST /internal/v1/notifications/events
```

Through the gateway prefix with `/api/v1/notifications` (stripped):
`GET /api/v1/notifications`, …

Errors use `{"error": {"code": "...", "message": "..."}}`.

## Event intake

```json
{
  "eventId": "payment-event-123",
  "type": "payment.captured",
  "userId": "uuid",
  "data": {"email": "user@example.com", "amount": "₹999"}
}
```

New events → `201`; replays → `200 {duplicate: true}`. Unknown types
→ `400`. The handler maps types to default content/channels/priority;
DB templates override rendering per channel at send time.

## Idempotency

`event_id` unique partial index: concurrent identical events race
safely into one notification ( losers adopt the winner's row). The
event handler is transport-independent — HTTP today, NATS JetStream
later — calling the same `Handle` with the same struct.

## Preferences

Defaults (no row): everything on except marketing. Updates coerce
`security_enabled` to true — password resets, suspicious logins, and
security alerts bypass every toggle. Type→category mapping
(payment/order→payment, learning→learning, course→course_updates,
user/auth→security) gates channel rows at creation.

## Retry policy

Email failures schedule 1m → 5m → 15m → 1h, then terminally fail.
Invalid recipients fail immediately without provider calls. `ClaimDue`
(`FOR UPDATE SKIP LOCKED`) exposes the same path to the retry worker;
in-app rows flip to sent on store since persisting IS delivery.

## Local development

Console provider prints the exact dev email block to stdout; no real
mail leaves the machine unless `EMAIL_PROVIDER=smtp` is configured.

```bash
psql "$DATABASE_URL" -f migrations/001_create_notification_tables.sql
go run ./cmd/server
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_notification?... \
  go test -tags integration ./...
```

Unit tests use fake stores/providers; integration uses real PostgreSQL
(incl. 8-way duplicate-event race and retry-lifecycle tests).
