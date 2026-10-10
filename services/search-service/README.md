# Search Service

Owns the **unified search index**: a denormalized copy of
published courses, videos, shorts, posts, and creator channels
used only for discovery. The source services remain the
authoritative owners of their content; this service never
reads or writes another service's database.

Port: `8091` · Database: `edvance_search` · Module:
`github.com/Anshul563/edvance-project/services/search-service`

## Responsibilities

- `search_documents`: the unified, eventually-consistent
  full-text index (PostgreSQL `tsvector` + GIN)
- unified relevance search across all source types
- search suggestions (bounded title-prefix typeahead)
- trending searches (derived from recorded search events)
- internal indexing: idempotent upsert, delete, bounded reindex

## Non-responsibilities

Owning any content, auth, media, courses, enrollments, or
payments. The index is a read-optimized copy; source services
own the truth. A UUID in the index is **not** proof the source
object still exists — producers must delete or update the
document when content is removed or becomes ineligible.

## Environment

```env
APP_ENV=development
SEARCH_SERVICE_PORT=8091
DATABASE_URL=postgres://postgres:postgres@localhost:5432/edvance_search?sslmode=disable
JWT_ACCESS_SECRET=   # optional: public search is unauthenticated
JWT_ISSUER=edvance-auth
JWT_AUDIENCE=edvance-api
INTERNAL_SERVICE_TOKEN=   # guards /internal/v1/*; fail closed while empty
DEFAULT_PAGE_SIZE=20
MAX_PAGE_SIZE=50
SEARCH_MAX_QUERY_LENGTH=200
SEARCH_MAX_SUGGEST_LENGTH=50
SEARCH_MAX_REINDEX_BATCH=500
```

## Endpoints

Public search is mounted at the root (the gateway forwards
`/api/v1/search/*` with the prefix stripped) and under
`/api/v1/search` for direct callers.

```text
GET  /?q=&type=&category=&language=&page=&limit=&sort=
GET  /suggestions?q=
GET  /trending

GET  /api/v1/search?...            (same handlers, full path)
GET  /health  |  GET  /ready
```

- `type`: `all` (default) | `course` | `video` | `short` | `post` | `creator`
- `sort`: `relevance` (default) | `newest`
- pagination: `page` default 1, `limit` default 20, max 50

Internal indexing lives only at the root, is guarded by the
shared `INTERNAL_SERVICE_TOKEN` (fail closed), and is **never**
mounted by the API gateway:

```text
PUT    /internal/v1/search/documents/:type/:id   upsert (idempotent)
DELETE /internal/v1/search/documents/:type/:id   delete
POST   /internal/v1/search/reindex              bounded bulk upsert
```

Errors use `{"error": "..."}`.

## Search implementation

Relevance ranking uses PostgreSQL full-text search:

- `search_vector` is a **generated STORED** column, so it is
  always synchronized with `title`/`description`/`body` with no
  triggers. Title is weighted `A`, description `B`, body `C`, so
  title matches rank above body matches.
- Queries are parsed with `websearch_to_tsquery('english', $1)`
  as a **bound parameter**, so a query can never inject SQL.
- Only `visibility = 'public'` documents are returned; private,
  draft, unlisted, and any other value are excluded.
- Results are ordered by `ts_rank` (relevance) or
  `published_at` (newest), with deterministic tie-breaking on
  `published_at` then `title`.
- Filters (`source_type`, `category`, `language`) and
  pagination (`LIMIT`/`OFFSET`) are applied in SQL with bound
  parameters; the total count is returned in the same query
  shape.
- `sort` is selected from a fixed whitelist, so it can never
  reach SQL as a raw identifier.

Suggestions use a bounded prefix `title ILIKE 'prefix%'`
(supported by a `lower(title) text_pattern_ops` index), which
is the right semantics for typeahead and stays cheap.

## Search event tracking

Every search records a **normalized** (lowercased, whitespace-
collapsed) query plus its result count in `search_events`. No
user identity and no raw query text is stored. `/trending`
aggregates the last 24 hours of recorded events, so trending is
derived from real traffic, never hardcoded. Event recording is
best-effort and asynchronous: a tracking failure never fails the
search response.

## Indexing & eventual consistency

Because source services do not yet emit indexing events, the
index is maintained through the internal upsert/delete
endpoints and the bounded reindex endpoint. Producers call
`PUT` when content is created/updated and `DELETE` when content
is removed or becomes ineligible. `POST /reindex` accepts up to
`SEARCH_MAX_REINDEX_BATCH` documents so a producer can rebuild a
slice of its index in one call. Until producers are wired up,
the index reflects only what has been explicitly indexed.

## Authentication & authorization

- Public search is unauthenticated. An optional JWT middleware
  attaches a caller identity when a valid token is present but
  never rejects, so anonymous search keeps working.
- Internal indexing requires the shared `INTERNAL_SERVICE_TOKEN`
  (constant-time bearer comparison) and fails closed while it is
  empty.
- Internal requests cannot forge another service's documents:
  `source_type` and `source_id` come from the URL path, not the
  body.

## Local development

```bash
createdb edvance_search        # or: psql -c 'CREATE DATABASE edvance_search'
psql "$DATABASE_URL" -f migrations/001_create_search_tables.sql
cp .env.example .env          # set INTERNAL_SERVICE_TOKEN
go run ./cmd/server
```

## Testing

```bash
go test ./...
DATABASE_URL=postgres://.../edvance_search?... \
  go test -tags integration ./...
```
