CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Payment lifecycle owned by payment-service. commerce_order_id is a
-- cross-service identifier (plain UUID): deliberately NO foreign key to
-- the commerce-service database. Money is ALWAYS integer minor units
-- (paise) in BIGINT columns — never floating point.
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    commerce_order_id UUID NOT NULL,

    amount_cents BIGINT NOT NULL,

    currency VARCHAR(3) NOT NULL,

    status VARCHAR(30) NOT NULL DEFAULT 'created',

    provider VARCHAR(30) NOT NULL DEFAULT 'razorpay',

    provider_order_id VARCHAR(255),

    provider_payment_id VARCHAR(255),

    provider_signature TEXT,

    receipt VARCHAR(255),

    failure_code VARCHAR(100),

    failure_reason TEXT,

    metadata JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    authorized_at TIMESTAMPTZ,

    captured_at TIMESTAMPTZ,

    failed_at TIMESTAMPTZ,

    CONSTRAINT payments_status_check
        CHECK (
            status IN (
                'created',
                'authorized',
                'captured',
                'failed',
                'refunded',
                'partially_refunded',
                'cancelled'
            )
        ),

    CONSTRAINT payments_amount_check
        CHECK (amount_cents > 0)
);

CREATE TABLE IF NOT EXISTS refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    payment_id UUID NOT NULL REFERENCES payments (id) ON DELETE CASCADE,

    amount_cents BIGINT NOT NULL,

    currency VARCHAR(3) NOT NULL,

    status VARCHAR(30) NOT NULL DEFAULT 'created',

    provider_refund_id VARCHAR(255),

    reason TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    processed_at TIMESTAMPTZ,

    CONSTRAINT refunds_amount_check
        CHECK (amount_cents > 0),

    CONSTRAINT refunds_status_check
        CHECK (
            status IN (
                'created',
                'processed',
                'failed'
            )
        )
);

-- Idempotent webhook event store: (provider, event_id) is the replay
-- key so Razorpay retries never double-apply business effects.
CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    provider VARCHAR(30) NOT NULL,

    event_id VARCHAR(255),

    event_type VARCHAR(100) NOT NULL,

    payload JSONB NOT NULL,

    signature TEXT,

    status VARCHAR(20) NOT NULL DEFAULT 'received',

    error_message TEXT,

    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    processed_at TIMESTAMPTZ,

    CONSTRAINT webhook_events_status_check
        CHECK (
            status IN (
                'received',
                'processed',
                'failed'
            )
        )
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_order_id
    ON payments(provider_order_id)
    WHERE provider_order_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_payment_id
    ON payments(provider_payment_id)
    WHERE provider_payment_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_payments_user_id
    ON payments(user_id);

CREATE INDEX IF NOT EXISTS idx_payments_commerce_order_id
    ON payments(commerce_order_id);

CREATE INDEX IF NOT EXISTS idx_payments_status
    ON payments(status);

CREATE INDEX IF NOT EXISTS idx_payments_created_at
    ON payments(created_at);

CREATE INDEX IF NOT EXISTS idx_refunds_payment_id
    ON refunds(payment_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_refunds_provider_refund_id
    ON refunds(provider_refund_id)
    WHERE provider_refund_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_events_event_id
    ON webhook_events(provider, event_id)
    WHERE event_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_webhook_events_event_type
    ON webhook_events(event_type);

CREATE INDEX IF NOT EXISTS idx_webhook_events_received_at
    ON webhook_events(received_at);
