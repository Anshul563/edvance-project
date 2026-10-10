-- Search service schema.
--
-- search_documents is the service's own denormalized search
-- index. source_type/source_id are cross-service references
-- (plain UUIDs) with NO foreign keys: the source services
-- remain the authoritative owners of their content, and this
-- index is eventually consistent with them.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS search_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_type VARCHAR(32) NOT NULL,
    source_id UUID NOT NULL,
    owner_id UUID,
    title TEXT NOT NULL,
    description TEXT,
    body TEXT,
    thumbnail_url TEXT,
    canonical_url TEXT,
    visibility VARCHAR(16) NOT NULL DEFAULT 'public',
    language VARCHAR(8),
    category VARCHAR(64),
    tags JSONB NOT NULL DEFAULT '{}'::jsonb,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    published_at TIMESTAMPTZ,
    indexed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Generated STORED column: always synchronized with the
    -- indexed text, no triggers required. Title is weighted
    -- highest (A), description next (B), body last (C), so
    -- title matches rank above body matches.
    search_vector TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', COALESCE(title, '')), 'A') ||
        setweight(to_tsvector('english', COALESCE(description, '')), 'B') ||
        setweight(to_tsvector('english', COALESCE(body, '')), 'C')
    ) STORED,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT search_documents_source_unique UNIQUE (source_type, source_id),
    CONSTRAINT search_documents_source_type_check
        CHECK (source_type IN ('course', 'video', 'short', 'post', 'creator')),
    CONSTRAINT search_documents_visibility_check
        CHECK (visibility IN ('public', 'private', 'draft', 'unlisted'))
);

CREATE INDEX IF NOT EXISTS idx_search_documents_search_vector
    ON search_documents USING GIN (search_vector);

CREATE INDEX IF NOT EXISTS idx_search_documents_source_type
    ON search_documents (source_type);

CREATE INDEX IF NOT EXISTS idx_search_documents_owner_id
    ON search_documents (owner_id);

CREATE INDEX IF NOT EXISTS idx_search_documents_category
    ON search_documents (category);

CREATE INDEX IF NOT EXISTS idx_search_documents_published_at
    ON search_documents (published_at);

CREATE INDEX IF NOT EXISTS idx_search_documents_visibility
    ON search_documents (visibility);

-- Supports the bounded prefix ILIKE used by the suggestions
-- endpoint (title ILIKE 'prefix%').
CREATE INDEX IF NOT EXISTS idx_search_documents_title_pattern
    ON search_documents (lower(title) text_pattern_ops);

-- Lightweight search-event log for trending. Stores only a
-- normalized query and result count: no user identity and no
-- raw sensitive query text.
CREATE TABLE IF NOT EXISTS search_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    query_normalized TEXT NOT NULL,
    result_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_search_events_created_at
    ON search_events (created_at);

CREATE INDEX IF NOT EXISTS idx_search_events_query_normalized
    ON search_events (query_normalized);
