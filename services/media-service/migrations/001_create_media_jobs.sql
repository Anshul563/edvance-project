CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Media processing jobs owned by media-service. video_id is a
-- cross-service identifier (plain UUID): deliberately NO foreign key to
-- the video-service database. FFmpeg never runs here; the external
-- media engine does, and this table tracks the orchestration state.
CREATE TABLE IF NOT EXISTS media_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    video_id UUID NOT NULL,

    job_type VARCHAR(30) NOT NULL,

    status VARCHAR(30) NOT NULL DEFAULT 'queued',

    source_object_key TEXT,

    output_manifest_url TEXT,

    output_thumbnail_url TEXT,

    duration_seconds BIGINT,

    width INTEGER,

    height INTEGER,

    progress INTEGER NOT NULL DEFAULT 0,

    attempt_count INTEGER NOT NULL DEFAULT 0,

    idempotency_key VARCHAR(255),

    engine_job_id VARCHAR(255),

    error_code VARCHAR(100),

    error_message TEXT,

    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    started_at TIMESTAMPTZ,

    completed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT media_jobs_type_check
        CHECK (
            job_type IN (
                'video_transcode',
                'thumbnail_generate',
                'video_probe'
            )
        ),

    CONSTRAINT media_jobs_status_check
        CHECK (
            status IN (
                'queued',
                'running',
                'completed',
                'failed',
                'cancelled'
            )
        ),

    CONSTRAINT media_jobs_progress_check
        CHECK (
            progress >= 0 AND progress <= 100
        )
);

CREATE INDEX IF NOT EXISTS idx_media_jobs_video_id
    ON media_jobs(video_id);

CREATE INDEX IF NOT EXISTS idx_media_jobs_status
    ON media_jobs(status);

CREATE INDEX IF NOT EXISTS idx_media_jobs_engine_job_id
    ON media_jobs(engine_job_id);

CREATE INDEX IF NOT EXISTS idx_media_jobs_created_at
    ON media_jobs(created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_media_jobs_idempotency_key
    ON media_jobs(idempotency_key)
    WHERE idempotency_key IS NOT NULL;
