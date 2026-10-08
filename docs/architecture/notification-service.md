# Notification Service Architecture

## Position

```text
Auth Service ──┐
Course Service ─┤
Learning Svc ───┼──► Notification Service ──┬──► In-App (PostgreSQL)
Commerce Svc ───┤      (delivery infra)      └──► Email (provider)
Payment Svc ────┤
Video/Media ────┘
```

Notification-service is **delivery infrastructure**: it owns no
business-domain state and makes no business decisions. Originating
services decide *when* something happened; this service decides *how*
the user hears about it (channels, preferences, templates, retries).

## Data ownership

Its database holds only delivery state: notifications, per-channel
delivery rows, preferences, templates. `user_id` values are opaque
cross-service identifiers — no foreign keys outward, no reads inward.
User contact data (emails) arrives inside events; it is never looked
up from another service.

## Intake: HTTP today, NATS tomorrow

```text
Service
  ↓  POST /internal/v1/notifications/events (X-Internal-Token)
Notification Service: verify key → map type → render → tx
  (notification + delivery rows) → dispatch → 201 / 200 replay
```

The `event.Event` struct and `event.Handler` know no transport. When
NATS JetStream arrives, a subscriber calls the same `Handle` with the
same struct; the HTTP route becomes one thin adapter among (eventually)
one. No business logic moves.

## Delivery model

```text
Notification (the fact; exactly one per event id)
   ├── in_app → sent (persisting IS delivery)
   └── email  → pending → sent | failed → retry → sent/failed
```

One notification fans out to N delivery rows, each with independent
state. A failed email never blocks or duplicates the in-app row, and a
duplicate event never creates a second notification: the `event_id`
unique index plus race-adopt logic converge all concurrent deliveries.

## Why preferences gate creation, not sending

Disabled channels produce no delivery rows at all — there is nothing to
retry, cancel, or leak later. Security types skip the gate entirely, so
a user with everything off still receives password resets and alerts.

## Retry model

Attempt schedule 1m → 5m → 15m → 1h, max attempts configurable
(`NOTIFICATION_MAX_ATTEMPTS`, default 4), then terminally failed.
An in-process worker (`internal/worker`, N pollers × interval, clean
shutdown) claims due rows via `ClaimDue` (`FOR UPDATE SKIP LOCKED`) so
pollers never double-send or block. Invalid recipients fail immediately
(no provider call, no retries): permanent failures must not consume
the schedule.

## Event mapping

`internal/event` splits defaults per domain (`auth.go`, `course.go`,
`learning.go`, `payment.go`, `video.go`); `handler.go` only merges and
drives creation. Copy follows the spec's example titles/bodies with
`{{.variable}}` interpolation; missing variables fail loudly instead of
sending broken copy.

## Push/SMS later

Channels exist in the domain (`push`, `sms`) with no providers wired.
Adding FCM means: a provider implementation, a `DispatchNew` branch,
and preference toggles — no schema or domain changes.

## Deliberate limitations (v1)

- Sends happen inline at creation; the worker covers scheduled retries.
- No WebSockets/FCM/SMS, no template admin UI, no marketing types.
- Console email prints bodies to stdout — dev only, forbidden in prod
  by config validation.
- No cross-service reads: recipient emails must arrive in events.
