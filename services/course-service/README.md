# Course Service

Owns the **educational course structure**: courses, sections, lessons,
ordering, publishing state, objectives, and requirements.

Port: `8086` · Database: `edvance_course` · Module:
`github.com/Anshul563/edvance-project/services/course-service`

## Responsibilities

- courses (metadata, slug, level, language, visibility, price placeholder)
- sections with dense server-side positions
- lessons with types (`video`, `article`, `quiz`, `assignment`,
  `coding`, `resource`, `live`) and ordering
- learning objectives and requirements
- publish / archive lifecycle
- public vs owner visibility

## Non-responsibilities

Users, creator profiles, auth, video processing, FFmpeg, storage,
enrollments, progress, certificates, comments, reviews, payments,
recommendations, search, analytics, quiz/coding engines. Lesson
`content_id` values are opaque cross-service references (to
content-service → video-service), never joined or fetched here.

## Environment

```env
APP_ENV=development
COURSE_SERVICE_PORT=8086
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_course?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
```

## Endpoints

Through the gateway prefix with `/api/v1` (the gateway strips only
`/api/v1` for the three mounts, so paths arrive intact):

```text
POST   /api/v1/courses                        create (draft, slug generated)
GET    /api/v1/courses/:courseID              owner full / public published
PATCH  /api/v1/courses/:courseID              partial update (owner)
POST   /api/v1/courses/:courseID/publish      draft -> published (validated)
POST   /api/v1/courses/:courseID/archive      -> archived
GET    /api/v1/courses/:courseID/structure    curriculum (preview-filtered)
GET    /api/v1/courses/creator/:creatorID?page=&limit=&status=
POST   /api/v1/courses/:courseID/objectives   | GET | DELETE /objectives/:id
POST   /api/v1/courses/:courseID/requirements | GET | DELETE /requirements/:id
POST   /api/v1/courses/:courseID/sections
POST   /api/v1/courses/:courseID/sections/reorder   {sectionIds: [...]}
PATCH  /api/v1/sections/:sectionID
DELETE /api/v1/sections/:sectionID            (cascades lessons, compacts)
POST   /api/v1/sections/:sectionID/lessons
POST   /api/v1/sections/:sectionID/lessons/reorder {lessonIds: [...]}
PATCH  /api/v1/lessons/:lessonID
DELETE /api/v1/lessons/:lessonID              (compacts)
GET    /health  |  GET /ready
```

Errors use `{"error": {"code": "...", "message": "..."}}`.

## Authentication & authorization

JWTs validated locally (HS256-only, issuer/audience/expiry); `sub` is
the identity. `CreatorAuthorization.CanManageCreator` is an isolated
seam: v1 trusts the JWT identity, later replaced by internal gRPC to
creator-service. `user_id` is never assumed to equal `creator_id`.

## Lifecycle & ordering

- Courses: `draft -> published -> archived` (no other moves; no delete).
- Positions are dense (`0..n-1`), server-assigned (`MAX+1` under a
  parent row lock), rewritten transactionally on reorder/delete.
- Publish requires: draft status, ≥1 section, ≥1 lesson, valid title,
  description, ≥1 objective (all reasons reported at once).
- Public sees `published + public` only; drafts/private/archived read
  as 404. Public curriculum redacts non-preview lessons.

## Local development

```bash
psql "$DATABASE_URL" -f migrations/001_create_courses.sql
go run ./cmd/server
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_course?... \
  go test -tags integration ./...
```
