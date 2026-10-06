CREATE TABLE IF NOT EXISTS email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,

    -- SHA-256 hex of the opaque verification token. The raw token is never stored.
    token_hash TEXT NOT NULL,

    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verification_tokens_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_user_id
    ON email_verification_tokens (user_id);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_token_hash
    ON email_verification_tokens (token_hash);

CREATE INDEX IF NOT EXISTS idx_email_verification_tokens_expires_at
    ON email_verification_tokens (expires_at);
