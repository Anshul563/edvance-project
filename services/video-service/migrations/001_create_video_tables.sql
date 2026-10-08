-- Video Service: media asset pipeline.
--
-- Supersedes the previous single-table `videos` design (content-linked
-- playback records). The upload/processing lifecycle now lives on
-- media_assets, with variants, jobs, thumbnails and captions as
-- separate tables. The legacy table is dropped; it held no rows in
-- any environment.
--
-- FFmpeg never runs here. This service owns metadata and orchestration;
-- the external media engine owns processing.

DROP TABLE IF EXISTS videos;

-- ---------------------------------------------------------------------------
-- media_assets: one row per uploaded object. The status column carries the
-- upload/processing state machine:
--   created -> uploading -> uploaded -> processing -> ready
--   processing -> failed, failed -> processing, * -> deleted
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS media_assets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id            UUID NOT NULL,
    type                VARCHAR(10) NOT NULL DEFAULT 'video'
                        CONSTRAINT media_assets_type_check
                        CHECK (type IN ('video', 'audio', 'image')),
    original_filename   TEXT NOT NULL,
    mime_type           TEXT NOT NULL,
    file_size_bytes     BIGINT NOT NULL CONSTRAINT media_assets_size_check
                        CHECK (file_size_bytes > 0),
    storage_key         TEXT NOT NULL CONSTRAINT media_assets_storage_key_unique
                        UNIQUE,
    status              VARCHAR(16) NOT NULL DEFAULT 'created'
                        CONSTRAINT media_assets_status_check
                        CHECK (status IN (
                            'created', 'uploading', 'uploaded',
                            'processing', 'ready', 'failed', 'deleted'
                        )),
    duration_seconds    BIGINT,
    width               INTEGER,
    height              INTEGER,
    frame_rate          NUMERIC(8, 3),
    codec               VARCHAR(32),
    bitrate             BIGINT,
    container           VARCHAR(16),
    playback_url        TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at        TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_media_assets_owner_id
    ON media_assets (owner_id);
CREATE INDEX IF NOT EXISTS idx_media_assets_status
    ON media_assets (status);
CREATE INDEX IF NOT EXISTS idx_media_assets_created_at
    ON media_assets (created_at);

-- ---------------------------------------------------------------------------
-- media_variants: renditions produced by the engine. Not every quality is
-- produced for every upload, and (media_asset_id, quality) is unique so a
-- redelivered callback can never duplicate a rendition.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS media_variants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id  UUID NOT NULL REFERENCES media_assets (id)
                    ON DELETE CASCADE,
    quality         VARCHAR(16) NOT NULL
                    CONSTRAINT media_variants_quality_check
                    CHECK (quality IN (
                        '144p', '240p', '360p', '480p', '720p',
                        '1080p', '1440p', '2160p'
                    )),
    width           INTEGER NOT NULL,
    height          INTEGER NOT NULL,
    bitrate         BIGINT,
    codec           VARCHAR(32),
    container       VARCHAR(16),
    storage_key     TEXT NOT NULL,
    playback_url    TEXT,
    file_size_bytes BIGINT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT media_variants_asset_quality_unique
        UNIQUE (media_asset_id, quality)
);

CREATE INDEX IF NOT EXISTS idx_media_variants_media_asset_id
    ON media_variants (media_asset_id);

-- ---------------------------------------------------------------------------
-- processing_jobs: queue for engine work and storage cleanup. Claimed with
-- FOR UPDATE SKIP LOCKED; attempts/error_message drive the retry policy.
-- payload carries job parameters (thumbnail timestamp, cleanup scope).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS processing_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id  UUID NOT NULL REFERENCES media_assets (id)
                    ON DELETE CASCADE,
    job_type        VARCHAR(16) NOT NULL
                    CONSTRAINT processing_jobs_type_check
                    CHECK (job_type IN (
                        'probe', 'transcode', 'thumbnail',
                        'hls', 'caption', 'cleanup'
                    )),
    status          VARCHAR(16) NOT NULL DEFAULT 'queued'
                    CONSTRAINT processing_jobs_status_check
                    CHECK (status IN (
                        'queued', 'processing', 'completed',
                        'failed', 'cancelled'
                    )),
    attempts        INTEGER NOT NULL DEFAULT 0,
    priority        INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,
    payload         JSONB NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_processing_jobs_status
    ON processing_jobs (status);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_priority
    ON processing_jobs (priority);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_created_at
    ON processing_jobs (created_at);
CREATE INDEX IF NOT EXISTS idx_processing_jobs_media_asset_id
    ON processing_jobs (media_asset_id);
-- Hot path for the worker claim: queued jobs ordered by priority/age.
CREATE INDEX IF NOT EXISTS idx_processing_jobs_claim
    ON processing_jobs (priority DESC, created_at ASC)
    WHERE status = 'queued';

-- ---------------------------------------------------------------------------
-- thumbnails: still frames. At most ONE primary thumbnail per asset,
-- enforced by a partial unique index.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS thumbnails (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id    UUID NOT NULL REFERENCES media_assets (id)
                      ON DELETE CASCADE,
    storage_key       TEXT NOT NULL,
    url               TEXT,
    width             INTEGER,
    height            INTEGER,
    timestamp_seconds NUMERIC(10, 3),
    is_primary        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT thumbnails_asset_key_unique
        UNIQUE (media_asset_id, storage_key)
);

CREATE INDEX IF NOT EXISTS idx_thumbnails_media_asset_id
    ON thumbnails (media_asset_id);

CREATE UNIQUE INDEX IF NOT EXISTS thumbnails_one_primary_per_asset
    ON thumbnails (media_asset_id)
    WHERE is_primary;

-- ---------------------------------------------------------------------------
-- captions: uploaded subtitle tracks (no speech-to-text yet).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS captions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    media_asset_id  UUID NOT NULL REFERENCES media_assets (id)
                    ON DELETE CASCADE,
    language        VARCHAR(16) NOT NULL,
    label           TEXT NOT NULL,
    format          VARCHAR(8) NOT NULL
                    CONSTRAINT captions_format_check
                    CHECK (format IN ('vtt', 'srt')),
    storage_key     TEXT NOT NULL,
    url             TEXT,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT captions_asset_key_unique
        UNIQUE (media_asset_id, storage_key)
);

CREATE INDEX IF NOT EXISTS idx_captions_media_asset_id
    ON captions (media_asset_id);

CREATE UNIQUE INDEX IF NOT EXISTS captions_one_default_per_asset
    ON captions (media_asset_id)
    WHERE is_default;
