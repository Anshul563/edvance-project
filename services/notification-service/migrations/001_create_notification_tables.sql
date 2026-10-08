CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- In-app notifications owned by notification-service. user_id is a
-- cross-service identifier (plain UUID): deliberately NO foreign keys
-- to other services' databases. event_id is the cross-service
-- idempotency key: retried events converge instead of duplicating.
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    type VARCHAR(100) NOT NULL,

    title VARCHAR(255) NOT NULL,

    body TEXT NOT NULL,

    data JSONB,

    priority VARCHAR(20) NOT NULL DEFAULT 'normal',

    event_id VARCHAR(255),

    read_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT notifications_priority_check
        CHECK (
            priority IN (
                'low',
                'normal',
                'high',
                'critical'
            )
        )
);

-- One delivery record per (notification, channel). The notification row
-- is the domain fact; deliveries track per-channel send state so email
-- can fail/retry independently of the stored in-app notification.
CREATE TABLE IF NOT EXISTS notification_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    notification_id UUID NOT NULL REFERENCES notifications (id) ON DELETE CASCADE,

    channel VARCHAR(20) NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    attempt_count INTEGER NOT NULL DEFAULT 0,

    provider_message_id VARCHAR(255),

    error_code VARCHAR(100),

    error_message TEXT,

    next_retry_at TIMESTAMPTZ,

    sent_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT notification_deliveries_channel_check
        CHECK (
            channel IN (
                'in_app',
                'email',
                'push',
                'sms'
            )
        ),

    CONSTRAINT notification_deliveries_status_check
        CHECK (
            status IN (
                'pending',
                'processing',
                'sent',
                'failed',
                'cancelled'
            )
        ),

    CONSTRAINT notification_deliveries_unique
        UNIQUE (notification_id, channel)
);

CREATE TABLE IF NOT EXISTS notification_preferences (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL UNIQUE,

    email_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    in_app_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    marketing_enabled BOOLEAN NOT NULL DEFAULT FALSE,

    course_updates_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    learning_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    payment_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    security_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    creator_enabled BOOLEAN NOT NULL DEFAULT TRUE,

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS notification_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    type VARCHAR(100) NOT NULL,

    channel VARCHAR(20) NOT NULL,

    subject_template TEXT,

    title_template TEXT,

    body_template TEXT NOT NULL,

    version INTEGER NOT NULL DEFAULT 1,

    active BOOLEAN NOT NULL DEFAULT TRUE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT notification_templates_channel_check
        CHECK (
            channel IN (
                'email',
                'in_app'
            )
        ),

    CONSTRAINT notification_templates_unique
        UNIQUE (type, channel, version)
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id
    ON notifications(user_id);

CREATE INDEX IF NOT EXISTS idx_notifications_created_at
    ON notifications(created_at);

CREATE INDEX IF NOT EXISTS idx_notifications_user_unread
    ON notifications(user_id, read_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_event_id
    ON notifications(event_id)
    WHERE event_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_notification_id
    ON notification_deliveries(notification_id);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_status
    ON notification_deliveries(status);

CREATE INDEX IF NOT EXISTS idx_notification_deliveries_retry
    ON notification_deliveries(next_retry_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_notification_preferences_user_id
    ON notification_preferences(user_id);

CREATE INDEX IF NOT EXISTS idx_notification_templates_type
    ON notification_templates(type, channel, active);
