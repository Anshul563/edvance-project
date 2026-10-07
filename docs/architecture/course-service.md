# Course Service Architecture

## Boundary

```text
creator-service
      │  owns creator identity + channel
      ▼
course-service
      │  owns the EDUCATIONAL CONTAINER:
      │  course → sections → lessons (+ objectives, requirements)
      ▼  lessons reference content by opaque ID:
Lesson (type=video)
      │  content_id ──► content-service (what the content is)
      ▼
video-service (media representation)
      │
      ▼
media-service (processing orchestration)
      │
      ▼
media-engine (FFmpeg execution)
```

The important distinction:

- **course-service** stores the structure: titles, order, types,
  `content_id` *references*.
- It never stores or duplicates video metadata, playback URLs, or
  content bodies. A lesson points at content; everything downstream
  resolves through the owning services.

## Data ownership

Each service owns its database. `creator_id` and lesson `content_id`
are opaque UUIDs — no cross-service foreign keys, no cross-service
JOINs. Objectives, requirements, sections, and lessons belong to this
database, so they ARE relationally connected here (`ON DELETE CASCADE`
within the service).

## Identity flow

```text
Client
  ↓  Authorization: Bearer <JWT from auth-service>
API Gateway  (strips /api/v1 only for the three mounts; no logic)
  ↓  /courses/*, /sections/*, /lessons/* arrive intact
Course Service (validates JWT locally; sub → user_id)
  ↓  CreatorAuthorization.CanManageCreator
     (v1: trust JWT identity · later: gRPC to creator-service)
PostgreSQL (edvance_course)
```

## Gateway routing note

Three gateway mounts (`/courses`, `/sections`, `/lessons`) share one
service. Stripping each mount fully would collapse
`/api/v1/sections/:id` and `/api/v1/courses/:id` into the same `/:id`,
so all three mounts strip only `/api/v1` and the service registers
full paths. This differs from single-mount services out of necessity,
not preference.

## Ordering model

Positions are dense integers assigned server-side:

- create: `MAX(position)+1` inside a transaction holding the parent
  row lock (`SELECT … FOR UPDATE`), so concurrent appends serialize
  instead of colliding.
- reorder: exact-set validation (same members, no dupes, nothing
  missing), then rewrite `0..n-1` in one transaction.
- delete: remove + renumber survivors in the same transaction.

No client-supplied positions are ever trusted.

## Publishing model

Publishing is a validation gate, not just a flag flip:

```text
draft  →  published   (sections ≥ 1, lessons ≥ 1, title valid,
                       description present, objectives ≥ 1;
                       status flip under row lock)
published → archived  (terminal; no deletes exist)
```

All failure reasons return at once (`COURSE_NOT_PUBLISHABLE`).

## Visibility model

- Owner (JWT identity manages the creator): everything, including
  drafts, full curriculum, and status-filtered listings.
- Everyone else: `published + public` only; everything else reads as
  404 (no existence oracle). Public curriculum redacts non-preview
  lessons to id/title/type/position; preview lessons stay complete.

## Future consumers (NOT implemented)

`learning-service` (enrollment, progress, certificates),
`assessment-service` (quiz/coding engines — lessons are placeholders),
`commerce-service` / `payment-service` (price fields are placeholders),
`review-service`, `search-service`. None of them require schema
changes here: lessons already carry stable IDs, types, preview flags,
and content references.
