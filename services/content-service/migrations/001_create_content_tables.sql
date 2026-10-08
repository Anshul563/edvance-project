CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Creator-generated content metadata, owned by content-service.
-- creator_id and media_asset_id are cross-service identifiers (plain
-- UUIDs): there are deliberately NO foreign keys to creator-service or
-- video-service databases. This service stores references only; media
-- files, transcoding, and storage live elsewhere.

CREATE TABLE IF NOT EXISTS videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    slug VARCHAR(250) NOT NULL,
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',

    media_asset_id UUID,
    thumbnail_url TEXT,

    duration_seconds INTEGER,
    view_count BIGINT NOT NULL DEFAULT 0,
    like_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,

    published_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT videos_slug_unique UNIQUE (slug),

    CONSTRAINT videos_visibility_check
        CHECK (visibility IN ('private', 'unlisted', 'public')),

    CONSTRAINT videos_status_check
        CHECK (
            status IN (
                'draft',
                'processing',
                'ready',
                'published',
                'archived'
            )
        ),

    CONSTRAINT videos_duration_check
        CHECK (
            duration_seconds IS NULL
            OR duration_seconds >= 0
        )
);

CREATE INDEX IF NOT EXISTS idx_videos_creator_id ON videos (creator_id);
CREATE INDEX IF NOT EXISTS idx_videos_status ON videos (status);
CREATE INDEX IF NOT EXISTS idx_videos_visibility ON videos (visibility);
CREATE INDEX IF NOT EXISTS idx_videos_published_at ON videos (published_at);
CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos (created_at);

CREATE TABLE IF NOT EXISTS shorts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    slug VARCHAR(250) NOT NULL,
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',

    media_asset_id UUID,
    thumbnail_url TEXT,

    duration_seconds INTEGER,
    view_count BIGINT NOT NULL DEFAULT 0,
    like_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,

    published_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT shorts_slug_unique UNIQUE (slug),

    CONSTRAINT shorts_visibility_check
        CHECK (visibility IN ('private', 'unlisted', 'public')),

    CONSTRAINT shorts_status_check
        CHECK (
            status IN (
                'draft',
                'processing',
                'ready',
                'published',
                'archived'
            )
        ),

    -- Platform limit; enforced in code as a configurable value
    -- (MAX_SHORT_DURATION_SECONDS) with this constraint as the
    -- database-level backstop.
    CONSTRAINT shorts_duration_check
        CHECK (
            duration_seconds IS NULL
            OR (
                duration_seconds >= 0
                AND duration_seconds <= 180
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_shorts_creator_id ON shorts (creator_id);
CREATE INDEX IF NOT EXISTS idx_shorts_status ON shorts (status);
CREATE INDEX IF NOT EXISTS idx_shorts_visibility ON shorts (visibility);
CREATE INDEX IF NOT EXISTS idx_shorts_published_at ON shorts (published_at);
CREATE INDEX IF NOT EXISTS idx_shorts_created_at ON shorts (created_at);

CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL,
    content TEXT NOT NULL,
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',

    like_count BIGINT NOT NULL DEFAULT 0,
    comment_count BIGINT NOT NULL DEFAULT 0,

    published_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT posts_visibility_check
        CHECK (visibility IN ('private', 'public')),

    CONSTRAINT posts_status_check
        CHECK (status IN ('draft', 'published', 'archived')),

    CONSTRAINT posts_content_check
        CHECK (length(btrim(content)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_posts_creator_id ON posts (creator_id);
CREATE INDEX IF NOT EXISTS idx_posts_status ON posts (status);
CREATE INDEX IF NOT EXISTS idx_posts_visibility ON posts (visibility);
CREATE INDEX IF NOT EXISTS idx_posts_published_at ON posts (published_at);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts (created_at);

-- Tags are global, normalized (lowercase name, kebab slug) so
-- capitalization differences can never create duplicates.
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    slug VARCHAR(120) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT tags_name_unique UNIQUE (name),
    CONSTRAINT tags_slug_unique UNIQUE (slug)
);

-- Hierarchical categories: self-referencing parent_id.
CREATE TABLE IF NOT EXISTS categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(120) NOT NULL,
    slug VARCHAR(140) NOT NULL,
    description TEXT,
    parent_id UUID REFERENCES categories (id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT categories_slug_unique UNIQUE (slug)
);

CREATE INDEX IF NOT EXISTS idx_categories_parent_id ON categories (parent_id);

-- Content <-> tag join tables. Composite primary keys guarantee a tag
-- is assigned to a given content row at most once.
CREATE TABLE IF NOT EXISTS video_tags (
    content_id UUID NOT NULL REFERENCES videos (id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, tag_id)
);

CREATE TABLE IF NOT EXISTS short_tags (
    content_id UUID NOT NULL REFERENCES shorts (id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, tag_id)
);

CREATE TABLE IF NOT EXISTS post_tags (
    content_id UUID NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (content_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_video_tags_tag_id ON video_tags (tag_id);
CREATE INDEX IF NOT EXISTS idx_short_tags_tag_id ON short_tags (tag_id);
CREATE INDEX IF NOT EXISTS idx_post_tags_tag_id ON post_tags (tag_id);
