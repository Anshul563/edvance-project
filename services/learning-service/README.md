# Learning Service

Owns **student learning state**: enrollments, lesson progress, course
progress math, resume pointers, and learning activity history.

Port: `8087` · Database: `edvance_learning` · Module:
`github.com/Anshul563/edvance-project/services/learning-service`

## Responsibilities

- enrollments (`active/completed/cancelled/suspended`; sources
  `free/manual` — purchase/subscription/gift later via commerce)
- lesson progress (monotonic percent, free position seeks, thresholds)
- lesson completion (idempotent) and course completion detection
- computed course progress (`completed/total*100`, never stored)
- resume pointers (last-incomplete → first-incomplete → first lesson)
- dashboard aggregates and append-only activity history

## Non-responsibilities

Users, auth, courses/sections/lessons, video metadata/processing,
payments, certificates, quizzes, reviews, comments, recommendations.
`course_id`/`lesson_id` are opaque cross-service references — never
joined, never foreign-keyed.

## Environment

```env
APP_ENV=development
LEARNING_SERVICE_PORT=8087
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_learning?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
COURSE_SERVICE_URL=http://localhost:8086
LESSON_COMPLETION_PERCENT=90
```

## Endpoints (all JWT-authed, strictly per-user)

```text
POST   /courses/:courseID/enroll
GET    /courses/:courseID/enrollment
GET    /me/courses?page=&limit=&status=
POST   /courses/:courseID/lessons/:lessonID/start
PATCH  /courses/:courseID/lessons/:lessonID/progress
POST   /courses/:courseID/lessons/:lessonID/complete
GET    /courses/:courseID/resume
GET    /courses/:courseID/progress
GET    /me/dashboard
GET    /me/activity?page=&limit=
GET    /health  |  GET /ready
```

Through the gateway prefix with `/api/v1/learning` (stripped):
`POST /api/v1/learning/courses/:id/enroll`, …

Errors use `{"error": {"code": "...", "message": "..."}}`.

## Authentication & isolation

JWTs validated locally; `sub` scopes every operation. One user can
never read or mutate another's enrollments, progress, activity, or
dashboard (tested).

## Enrollment rules

Free flow only (`priceCents == 0` + published + public, verified live
against course-service). Duplicates return the existing enrollment.
Paid/unpublished/missing courses fail coded (`COURSE_REQUIRES_PURCHASE`
409, `COURSE_NOT_PUBLISHED` 422, `COURSE_NOT_FOUND` 404). `EnrollUser`
is the internal/manual path for future commerce use (no public route).

## Progress rules

- Percent is monotonic: `max(current, requested)` applied in SQL-row
  locks, so concurrent reports converge instead of regressing.
- `last_position_seconds` seeks freely (video scrubbing).
- Threshold (`LESSON_COMPLETION_PERCENT`, default 90) flips lessons to
  completed with exactly one activity; explicit complete forces 100%.
- Repeats are idempotent (no resets, no duplicate activities).
- Course completes when all curriculum lessons complete
  (`enrollment → completed` + activity, same transaction).

## Course-service integration (`internal/course`)

Read-only HTTP client (10s timeouts) for course + structure reads used
in enrollment validation, lesson membership checks, progress math, and
resume ordering. 404 → domain not-found; anything else → 502
`COURSE_UNAVAILABLE` without leaking internals. No writes, no DB
access, ever.

## Local development

```bash
psql "$DATABASE_URL" -f migrations/001_create_learning_tables.sql
go run ./cmd/server
```

Course-service must run for anything beyond health checks.

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_learning?... \
  go test -tags integration ./...
```

Unit tests use fake stores/client; integration uses real PostgreSQL
plus an in-test fake course catalog.
