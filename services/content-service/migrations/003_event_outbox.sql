-- Transactional outbox for durable event publishing owned by
-- content-service.
--
-- Publishing a video or post must not fail because the notification
# bus was unavailable, and an event must never be lost between the
-- database commit and its JetStream publication. This table holds one
-- row per pending event inside the same transaction as the business
-- change. A background outbox publisher claims rows, publishes them to
-- NATS JetStream, and marks them published only after a server
-- acknowledgment.
--
-- content-service owns this table; no other service reads it directly.
CREATE TABLE IF NOT EXISTS event_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    event_type VARCHAR(200) NOT NULL,
    subject  VARCHAR(200) NOT NULL,
    payload  JSONB NOT NULL,

    status     VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts   INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ
);

-- Only pending rows are claimed; ordered by creation so older events
-- go first. SKIP LOCKED lets the publisher loop and any replicas claim
-- distinct rows without coordinating.
CREATE INDEX IF NOT EXISTS idx_event_outbox_pending
    ON event_outbox (status, created_at)
    WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_event_outbox_created_at
    ON event_outbox (created_at);
