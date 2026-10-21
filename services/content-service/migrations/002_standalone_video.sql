-- Standalone video experience, owned by content-service.
--
-- content-service already owns video metadata and publication. This
-- migration adds the fields the standalone (YouTube-style) surface
-- needs: language, taxonomy, a moderation state, and the processing
-- error/thumbnail details reported by the media pipeline.
--
-- No media bytes, storage keys, or engine jobs are stored here. Those
-- remain video-service's responsibility; the two services are linked by
-- media_asset_id only (no cross-database foreign key, by design).

ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS language VARCHAR(16) NOT NULL DEFAULT 'en';

ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS category_id UUID
        REFERENCES categories (id) ON DELETE SET NULL;

ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS moderation_status VARCHAR(20)
        NOT NULL DEFAULT 'approved';

ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS moderation_reason TEXT;

ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS processing_error_code VARCHAR(80);

ALTER TABLE videos
    ADD COLUMN IF NOT EXISTS processing_error_message TEXT;

-- Status lifecycle for standalone videos:
--   draft -> upload_pending -> uploaded -> processing -> ready
--   -> published (explicit creator action)
--   processing -> failed -> processing (retry)
--   ready -> archived (withdrawn)
ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_status_check;

ALTER TABLE videos
    ADD CONSTRAINT videos_status_check
    CHECK (
        status IN (
            'draft',
            'upload_pending',
            'uploaded',
            'processing',
            'ready',
            'published',
            'failed',
            'archived'
        )
    );

ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_moderation_status_check;

ALTER TABLE videos
    ADD CONSTRAINT videos_moderation_status_check
    CHECK (
        moderation_status IN (
            'approved',
            'pending_review',
            'rejected',
            'held'
        )
    );

ALTER TABLE videos DROP CONSTRAINT IF EXISTS videos_language_check;

ALTER TABLE videos
    ADD CONSTRAINT videos_language_check
    CHECK (
        language ~ '^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$'
    );

-- Public discovery reads the published+public+approved slice ordered by
-- publish time; the composite index serves it without a sort.
CREATE INDEX IF NOT EXISTS idx_videos_public_discovery
    ON videos (status, visibility, moderation_status, published_at DESC);

CREATE INDEX IF NOT EXISTS idx_videos_category_id ON videos (category_id);
CREATE INDEX IF NOT EXISTS idx_videos_language ON videos (language);
CREATE INDEX IF NOT EXISTS idx_videos_media_asset_id ON videos (media_asset_id);
CREATE INDEX IF NOT EXISTS idx_videos_moderation_status
    ON videos (moderation_status);
