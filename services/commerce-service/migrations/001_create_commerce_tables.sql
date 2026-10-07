CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Commercial purchase domain owned by commerce-service. course_id values
-- are cross-service identifiers (plain UUIDs): deliberately NO foreign
-- keys to other services' databases. Money is ALWAYS integer minor units
-- (paise/cents) in BIGINT columns — never floating point.
CREATE TABLE IF NOT EXISTS carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL UNIQUE,

    currency VARCHAR(3) NOT NULL DEFAULT 'INR',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cart_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    cart_id UUID NOT NULL REFERENCES carts (id) ON DELETE CASCADE,

    course_id UUID NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT cart_items_cart_course_unique
        UNIQUE (cart_id, course_id)
);

CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    order_number VARCHAR(40) NOT NULL UNIQUE,

    status VARCHAR(30) NOT NULL DEFAULT 'pending_payment',

    currency VARCHAR(3) NOT NULL DEFAULT 'INR',

    subtotal_cents BIGINT NOT NULL DEFAULT 0,

    discount_cents BIGINT NOT NULL DEFAULT 0,

    tax_cents BIGINT NOT NULL DEFAULT 0,

    total_cents BIGINT NOT NULL DEFAULT 0,

    coupon_code VARCHAR(50),

    payment_reference VARCHAR(255),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    completed_at TIMESTAMPTZ,

    cancelled_at TIMESTAMPTZ,

    CONSTRAINT orders_status_check
        CHECK (
            status IN (
                'pending_payment',
                'paid',
                'failed',
                'cancelled',
                'refunded',
                'partially_refunded'
            )
        ),

    CONSTRAINT orders_subtotal_check
        CHECK (subtotal_cents >= 0),

    CONSTRAINT orders_discount_check
        CHECK (discount_cents >= 0),

    CONSTRAINT orders_tax_check
        CHECK (tax_cents >= 0),

    CONSTRAINT orders_total_check
        CHECK (total_cents >= 0)
);

CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,

    course_id UUID NOT NULL,

    course_title_snapshot VARCHAR(200) NOT NULL,

    price_cents BIGINT NOT NULL,

    discount_cents BIGINT NOT NULL DEFAULT 0,

    final_price_cents BIGINT NOT NULL,

    currency VARCHAR(3) NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT order_items_price_check
        CHECK (price_cents >= 0),

    CONSTRAINT order_items_discount_check
        CHECK (discount_cents >= 0),

    CONSTRAINT order_items_final_price_check
        CHECK (final_price_cents >= 0)
);

-- Percentage values are basis points of a percent: 1000 = 10%,
-- 10000 = 100%. Integer math only, never floating point.
CREATE TABLE IF NOT EXISTS coupons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    code VARCHAR(50) NOT NULL UNIQUE,

    discount_type VARCHAR(20) NOT NULL,

    discount_value BIGINT NOT NULL,

    currency VARCHAR(3),

    minimum_order_cents BIGINT NOT NULL DEFAULT 0,

    maximum_discount_cents BIGINT,

    usage_limit INTEGER,

    used_count INTEGER NOT NULL DEFAULT 0,

    starts_at TIMESTAMPTZ NOT NULL,

    expires_at TIMESTAMPTZ,

    status VARCHAR(20) NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT coupons_discount_type_check
        CHECK (
            discount_type IN (
                'percentage',
                'fixed'
            )
        ),

    CONSTRAINT coupons_discount_value_check
        CHECK (discount_value >= 0),

    CONSTRAINT coupons_status_check
        CHECK (
            status IN (
                'active',
                'disabled',
                'expired'
            )
        ),

    CONSTRAINT coupons_usage_check
        CHECK (used_count >= 0)
);

CREATE TABLE IF NOT EXISTS coupon_usages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    coupon_id UUID NOT NULL REFERENCES coupons (id) ON DELETE CASCADE,

    user_id UUID NOT NULL,

    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT coupon_usages_unique
        UNIQUE (coupon_id, user_id, order_id)
);

-- A purchase records successful commercial ownership. It does NOT imply
-- the learning-service database was updated: enrollment provisioning
-- happens through an explicit, idempotent abstraction.
CREATE TABLE IF NOT EXISTS purchases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,

    course_id UUID NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'active',

    purchased_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    refunded_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT purchases_status_check
        CHECK (
            status IN (
                'active',
                'refunded',
                'revoked'
            )
        ),

    CONSTRAINT purchases_order_course_unique
        UNIQUE (order_id, course_id)
);

CREATE INDEX IF NOT EXISTS idx_cart_items_cart_id
    ON cart_items(cart_id);

CREATE INDEX IF NOT EXISTS idx_cart_items_course_id
    ON cart_items(course_id);

CREATE INDEX IF NOT EXISTS idx_orders_user_id
    ON orders(user_id);

CREATE INDEX IF NOT EXISTS idx_orders_status
    ON orders(status);

CREATE INDEX IF NOT EXISTS idx_orders_created_at
    ON orders(created_at);

CREATE INDEX IF NOT EXISTS idx_order_items_order_id
    ON order_items(order_id);

CREATE INDEX IF NOT EXISTS idx_order_items_course_id
    ON order_items(course_id);

CREATE INDEX IF NOT EXISTS idx_coupons_code
    ON coupons(code);

CREATE INDEX IF NOT EXISTS idx_coupons_status
    ON coupons(status);

CREATE INDEX IF NOT EXISTS idx_coupon_usages_coupon_id
    ON coupon_usages(coupon_id);

CREATE INDEX IF NOT EXISTS idx_coupon_usages_user_id
    ON coupon_usages(user_id);

CREATE INDEX IF NOT EXISTS idx_purchases_user_id
    ON purchases(user_id);

CREATE INDEX IF NOT EXISTS idx_purchases_course_id
    ON purchases(course_id);

CREATE INDEX IF NOT EXISTS idx_purchases_status
    ON purchases(status);
