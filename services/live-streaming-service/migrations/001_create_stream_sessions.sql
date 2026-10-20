-- Stream sessions owned by live-streaming-service. The service's own
-- database (edvance_live); no foreign keys across service databases —
-- creator_id is the auth-service identity (JWT sub), deliberately without
-- a constraint. Records are never physically deleted: ended sessions keep
-- their history.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS stream_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    creator_id UUID NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'waiting',
    -- MediaMTX path name; public identifier used in the HLS playback URL.
    path_name VARCHAR(64) NOT NULL,
    -- SHA-256 hex of the publisher token. The raw token is never stored.
    stream_key_hash VARCHAR(64) NOT NULL,
    -- Display-only hint (first characters of the token) so the creator can
    -- recognise the key without exposing it.
    stream_key_hint VARCHAR(16) NOT NULL,
    rtmp_url TEXT NOT NULL,
    playback_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    CONSTRAINT stream_sessions_status_check
        CHECK (status IN ('waiting', 'live', 'ended')),
    CONSTRAINT stream_sessions_title_check
        CHECK (char_length(title) BETWEEN 1 AND 200)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_stream_sessions_path_name
    ON stream_sessions (path_name);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_creator_id
    ON stream_sessions (creator_id);
CREATE INDEX IF NOT EXISTS idx_stream_sessions_status
    ON stream_sessions (status);
