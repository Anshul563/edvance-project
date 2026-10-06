CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Media representation of content, owned by video-service.
-- content_id and creator_id are cross-service identifiers (plain UUIDs):
-- there are deliberately NO foreign keys to other services' databases.
-- One video record belongs to exactly one content record (1:1).
CREATE TABLE IF NOT EXISTS videos (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    content_id UUID NOT NULL,
    creator_id UUID NOT NULL,

    status VARCHAR(30) NOT NULL DEFAULT 'pending',

    source_object_key TEXT,

    duration_seconds BIGINT,

    width INTEGER,
    height INTEGER,

    thumbnail_url TEXT,

    playback_manifest_url TEXT,

    processing_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT videos_status_check
        CHECK (
            status IN (
                'pending',
                'uploading',
                'processing',
                'ready',
                'failed',
                'deleted'
            )
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_videos_content_unique
    ON videos(content_id);

CREATE INDEX IF NOT EXISTS idx_videos_creator_id
    ON videos(creator_id);

CREATE INDEX IF NOT EXISTS idx_videos_status
    ON videos(status);

CREATE INDEX IF NOT EXISTS idx_videos_created_at
    ON videos(created_at);
