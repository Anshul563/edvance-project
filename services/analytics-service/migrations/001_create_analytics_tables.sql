CREATE TABLE IF NOT EXISTS analytics_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL UNIQUE,
    event_type VARCHAR(80) NOT NULL,
    event_version INTEGER NOT NULL DEFAULT 1,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    actor_id VARCHAR(128),
    anonymous_id VARCHAR(128),
    session_id VARCHAR(128),
    source_service VARCHAR(64) NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}',
    processing_status VARCHAR(32) NOT NULL DEFAULT 'pending',
    processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_analytics_events_occurred_at
    ON analytics_events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_analytics_events_event_type
    ON analytics_events (event_type);
CREATE INDEX IF NOT EXISTS idx_analytics_events_entity_ref
    ON analytics_events (entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_actor_ref
    ON analytics_events (actor_id);
CREATE INDEX IF NOT EXISTS idx_analytics_events_processing_status
    ON analytics_events (processing_status, occurred_at);

CREATE TABLE IF NOT EXISTS analytics_processing_checkpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    checkpoint_name VARCHAR(128) NOT NULL UNIQUE,
    last_event_id UUID,
    last_processed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS analytics_creator_daily (
    creator_id VARCHAR(128) NOT NULL,
    reporting_date DATE NOT NULL,
    content_views INTEGER NOT NULL DEFAULT 0,
    unique_viewers INTEGER NOT NULL DEFAULT 0,
    followers_gained INTEGER NOT NULL DEFAULT 0,
    course_enrollments INTEGER NOT NULL DEFAULT 0,
    course_completions INTEGER NOT NULL DEFAULT 0,
    watch_time_seconds BIGINT NOT NULL DEFAULT 0,
    revenue_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT analytics_creator_daily_pkey PRIMARY KEY (creator_id, reporting_date)
);

CREATE TABLE IF NOT EXISTS analytics_course_daily (
    course_id VARCHAR(128) NOT NULL,
    reporting_date DATE NOT NULL,
    course_views INTEGER NOT NULL DEFAULT 0,
    enrollments INTEGER NOT NULL DEFAULT 0,
    completions INTEGER NOT NULL DEFAULT 0,
    completion_rate NUMERIC(5,4) NOT NULL DEFAULT 0,
    lesson_activity INTEGER NOT NULL DEFAULT 0,
    watch_time_seconds BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT analytics_course_daily_pkey PRIMARY KEY (course_id, reporting_date)
);

CREATE TABLE IF NOT EXISTS analytics_platform_daily (
    reporting_date DATE NOT NULL,
    active_users INTEGER NOT NULL DEFAULT 0,
    new_enrollments INTEGER NOT NULL DEFAULT 0,
    course_completions INTEGER NOT NULL DEFAULT 0,
    published_content_count INTEGER NOT NULL DEFAULT 0,
    gross_revenue_cents BIGINT NOT NULL DEFAULT 0,
    refunds_cents BIGINT NOT NULL DEFAULT 0,
    net_revenue_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT analytics_platform_daily_pkey PRIMARY KEY (reporting_date)
);

CREATE TABLE IF NOT EXISTS analytics_search_daily (
    reporting_date DATE NOT NULL,
    search_count INTEGER NOT NULL DEFAULT 0,
    unique_searchers INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT analytics_search_daily_pkey PRIMARY KEY (reporting_date)
);
