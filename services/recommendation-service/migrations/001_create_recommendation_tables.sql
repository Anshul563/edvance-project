CREATE TABLE IF NOT EXISTS recommendation_items (
    id UUID PRIMARY KEY,
    source_type VARCHAR(32) NOT NULL,
    source_id UUID NOT NULL,
    creator_id UUID NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(128),
    tags JSONB,
    language VARCHAR(32),
    difficulty VARCHAR(64),
    visibility VARCHAR(32) NOT NULL DEFAULT 'public',
    published_at TIMESTAMPTZ,
    engagement_score DOUBLE PRECISION NOT NULL DEFAULT 0,
    metadata JSONB,
    is_eligible BOOLEAN NOT NULL DEFAULT TRUE,
    indexed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_recommendation_item_source
    ON recommendation_items (source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_source_type
    ON recommendation_items (source_type);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_creator
    ON recommendation_items (creator_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_category
    ON recommendation_items (category);
CREATE INDEX IF NOT EXISTS idx_recommendation_items_published_at
    ON recommendation_items (published_at DESC);

CREATE TABLE IF NOT EXISTS user_interests (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    category VARCHAR(128),
    tag VARCHAR(128),
    weight DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_interest_category
    ON user_interests (user_id, category)
    WHERE category IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_interest_tag
    ON user_interests (user_id, tag)
    WHERE tag IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_user_interests_user
    ON user_interests (user_id);

CREATE TABLE IF NOT EXISTS recommendation_events (
    id UUID PRIMARY KEY,
    user_id UUID NULL,
    anonymous_id UUID NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id UUID NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    event_weight DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    session_id UUID NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recommendation_events_user
    ON recommendation_events (user_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_events_source
    ON recommendation_events (source_type, source_id);
CREATE INDEX IF NOT EXISTS idx_recommendation_events_type
    ON recommendation_events (event_type);
CREATE INDEX IF NOT EXISTS idx_recommendation_events_created_at
    ON recommendation_events (created_at DESC);

CREATE TABLE IF NOT EXISTS recommendation_feedback (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id UUID NOT NULL,
    feedback_type VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_feedback_user_item_type
    ON recommendation_feedback (user_id, source_type, source_id, feedback_type);
CREATE INDEX IF NOT EXISTS idx_feedback_user
    ON recommendation_feedback (user_id);
CREATE INDEX IF NOT EXISTS idx_feedback_item
    ON recommendation_feedback (source_type, source_id);
