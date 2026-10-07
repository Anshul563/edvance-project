# Learning Service Architecture

## Boundary

```text
User
 ↓  JWT (auth-service issuance, local validation everywhere)
Learning Service
 ├── Enrollment        (who studies what, since when, from where)
 ├── Lesson Progress  (per-lesson traversal state)
 ├── Course Progress  (computed, never stored)
 └── Learning Activity(append-only event log)
          │
          ▼  reads only: course + structure
    Course Service
          │
          ├── Course
          ├── Section
          └── Lesson
```

Clearly stated:

```text
learning-service owns learning state.
course-service owns course structure.
Neither service accesses the other's database.
```

`user_id`, `course_id`, `lesson_id` cross boundaries as opaque UUIDs.
`enrollment_id` is local and relationally connected (`ON DELETE
CASCADE` for progress and activities).

## Enrollment flow

```text
Client
  ↓  POST /courses/:id/enroll
Learning Service: authenticate → dedupe check →
  course-service GET course → must be published + public + free →
  INSERT enrollment + enrolled activity (one tx)
  ↓  duplicate / race → return existing enrollment
```

Paid courses stop at `409 COURSE_REQUIRES_PURCHASE`. The future
commerce flow calls the internal `EnrollUser` path (sources `purchase`,
`subscription`, …) instead of this endpoint.

## Progress flow

```text
Client playback reports
  ↓  POST start (idempotent) / PATCH progress / POST complete
Learning Service: authenticate → own enrollment? →
  course-service structure → lesson ∈ course? →
  row-locked tx: progress math + enrollment touch + activity
  ↓  threshold crossed → lesson completed
  ↓  all lessons completed → enrollment completed (own tx)
```

Percent uses `max(current, requested)`; position seeks freely. All
multi-row mutations are single transactions: progress row, enrollment
touch, and activity insert commit or roll back together. Concurrent
reports serialize on the row lock, so `50%` can never overwrite `80%`.

## Progress math & resume

Course percent = `completed / total × 100`, computed live from lesson
rows + curriculum length — stored nowhere, so it cannot drift. Resume
prefers the last-accessed incomplete lesson, then the first incomplete
lesson in curriculum order, then the first lesson; completed courses
report `courseCompleted` instead of a lesson.

## Course-service dependency

Every mutation re-validates lesson identity against fresh structure
(the client is never trusted), and enrollment creation re-checks
course state. Upstream failures map to `502 COURSE_UNAVAILABLE`;
course-service is NOT a `/ready` dependency — learning state stays
readable during its outages (mutations degrade, reads of local state
succeed).

## Dashboard scope

Counts + recent pointers only (5 recent enrollments, 5 active to
continue). Per-course percentages are omitted on purpose: they would
cost one structure fetch per course. Recommendations, streaks, and
analytics belong to future services.

## Future consumers (NOT implemented)

`commerce-service` → `payment-service` → `EnrollUser`;
certificate service (reads completions + activities);
notifications, search, recommendations. None require schema changes
here: enrollments carry source/status, activities are a complete event
log, and progress rows are the single source of truth.
