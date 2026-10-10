ALTER TABLE refunds
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(200),
    ADD COLUMN IF NOT EXISTS request_fingerprint CHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS refunds_payment_idempotency_key_unique
    ON refunds (payment_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS payments_commerce_order_id_unique
    ON payments (commerce_order_id);
