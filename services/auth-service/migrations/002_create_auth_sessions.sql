CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,

    -- SHA-256 hex of the opaque refresh token. The raw token is never stored.
    refresh_token_hash TEXT NOT NULL,

    user_agent TEXT,
    ip_address TEXT,

    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    replaced_by_session_id UUID REFERENCES auth_sessions (id) ON DELETE SET NULL,

    CONSTRAINT auth_sessions_refresh_token_hash_unique UNIQUE (refresh_token_hash)
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user_id
    ON auth_sessions (user_id);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_refresh_token_hash
    ON auth_sessions (refresh_token_hash);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_expires_at
    ON auth_sessions (expires_at);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_revoked_at
    ON auth_sessions (revoked_at);
