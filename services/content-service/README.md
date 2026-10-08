# Content Service

Owns **creator content metadata**: videos, shorts, posts, the shared
tag vocabulary, and the category taxonomy — lifecycle, visibility, and
publishing state. Media bytes, transcoding, and storage belong to
video/media-service; this service stores references only.

Port: `8084` · Database: `edvance_content` · Module:
`github.com/Anshul563/edvance-project/services/content-service`

## Responsibilities

- videos (title, description, slug, media reference, thumbnail,
  duration, counters, publish state)
- shorts (same shape, plus the configurable duration ceiling)
- posts (text body, like/comment counters, publish state)
- tags (global, normalized vocabulary shared by all content)
- categories (read-only hierarchical taxonomy)
- visibility rules, publishing gates, and owner-only edits
- internal callbacks: media status, view/counter adjustments

## Non-responsibilities

Auth, creator profiles, media upload/transcoding/storage, courses,
enrollments, comments, likes-as-a-product-surface, search, analytics.
No other service's database is ever read or written: `creator_id` and
`media_asset_id` are opaque cross-service UUIDs with no foreign keys.

## Environment

```env
APP_ENV=development
CONTENT_SERVICE_PORT=8084
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_content?sslmode=disable
JWT_ACCESS_SECRET=   # must match auth-service (validate-only here)
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
CREATOR_SERVICE_URL=http://localhost:8083
NOTIFICATION_SERVICE_URL=http://localhost:8092
NOTIFICATION_INTERNAL_TOKEN=
INTERNAL_API_KEY=    # guards /internal/*; fail closed while empty
MAX_SHORT_DURATION_SECONDS=180
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=100
MAX_TAGS_PER_ITEM=10
```

## Endpoints

Content routes are mounted twice: at the root (the API gateway
forwards `/api/v1/content/*` with the prefix stripped) and under
`/api/v1/content` for direct callers.

```text
GET    /videos?page=&limit=&creator_id=&status=&visibility=
POST   /videos                              create (draft, slug generated)
GET    /videos/:videoID                     owner full / public published
PATCH  /videos/:videoID                     partial update (owner)
DELETE /videos/:videoID
POST   /videos/:videoID/publish             ready + media -> published
POST   /videos/:videoID/unpublish           published -> ready/private
GET    /creators/:creatorID/videos

GET    /shorts | POST /shorts               (same shape as videos)
GET    /shorts/:shortID | PATCH | DELETE
POST   /shorts/:shortID/publish | /unpublish
GET    /creators/:creatorID/shorts

GET    /posts | POST /posts                 posts have no slug/media
GET    /posts/:postID | PATCH | DELETE
POST   /posts/:postID/publish | /unpublish
GET    /creators/:creatorID/posts

GET    /tags | POST /tags | GET /tags/:slug
GET    /categories | GET /categories/:slug

GET    /health  |  GET /ready
```

Service-to-service routes live at the root only, never under the
gateway prefix, and demand the shared `INTERNAL_API_KEY` (fail closed):

```text
POST   /internal/videos/:videoID/status     {status,durationSeconds,mediaAssetId}
POST   /internal/videos/:videoID/view
POST   /internal/videos/:videoID/counters   {viewDelta,likeDelta,commentDelta}
POST   /internal/shorts/:shortID/status | /view | /counters
POST   /internal/posts/:postID/counters     {likeDelta,commentDelta}
```

Errors use `{"error": {"code": "...", "message": "..."}}`.

## Authentication & authorization

JWTs are validated locally (HS256-only, issuer/audience/expiry); `sub`
is the identity. Ownership is never taken from the request body: the
service forwards the caller's own bearer token to
`GET {CREATOR_SERVICE_URL}/creators/me` and uses the creator it gets
back.

- **Writes** fail closed: no JWT → 401, no creator profile → 403,
  someone else's row → 403, creator-service unreachable → 500.
- **Reads** degrade: if ownership cannot be resolved the viewer is
  treated as anonymous, so a creator-service outage never takes the
  catalog down and never leaks a draft.
- Server-owned fields (`creatorId`, `status`, counters, `publishedAt`)
  sent by a client are rejected with `FIELD_NOT_SETTABLE`.

## Lifecycle

- **videos / shorts:** `draft -> processing -> ready -> published`,
  plus `archived`. Media status arrives only over `/internal/*`, and
  the callback that says `ready` must also carry `mediaAssetId`
  (otherwise `400`) so a row can never sit ready with no asset.
  Unpublish returns `ready + private` and clears `published_at`.
- **posts:** `draft -> published` (unpublish returns `draft +
  private`). No media state.
- **Publish gates** (all reasons reported at once):
  - video: status `ready`, media asset present, title ≥ 3 chars
  - short: same, plus a duration within `MAX_SHORT_DURATION_SECONDS`
  - post: status `draft` and non-empty content within the limit
- Publishing runs `UPDATE ... WHERE status = <expected>` in a
  transaction, so racing publishes resolve to exactly one winner; the
  loser gets `NOT_PUBLISHABLE`.
- Slugs are server-generated and retried on collision; the `UNIQUE`
  constraint is the final arbiter.
- Tags are normalized (`"React   JS"` → `react js` / `react-js`) and
  deduplicated; duplicates across content rows resolve to one row.

## Visibility

`published + public` is readable by everyone; everything else is
owner-only and reads as **404** to non-owners (existence is not
leaked). List scope is the same rule, so an anonymous listing sees the
public catalog and a creator additionally sees their own rows.

## Local development

```bash
createdb edvance_content        # or: psql -c 'CREATE DATABASE edvance_content'
psql "$DATABASE_URL" -f migrations/001_create_content_tables.sql
cp .env.example .env            # set JWT_ACCESS_SECRET and INTERNAL_API_KEY
go run ./cmd/server
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_content?... \
  go test -tags integration ./...
```
