-- Standalone-video upload attestation.
--
-- Creators must confirm, at upload time, that the content is
-- educational and that they hold the rights to upload it. The
-- confirmations are recorded on the media asset so an audit can answer
-- "what did the uploader attest to, and when?" without trusting a
-- client-supplied flag later.
--
-- Enforcement lives in service.validateInit (video uploads only);
-- these columns are the durable record, not the check itself.

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS educational_confirmed_at TIMESTAMPTZ;

ALTER TABLE media_assets
    ADD COLUMN IF NOT EXISTS rights_confirmed_at TIMESTAMPTZ;

-- Owner listing and retry paths filter by owner+status.
CREATE INDEX IF NOT EXISTS idx_media_assets_owner_status
    ON media_assets (owner_id, status);
