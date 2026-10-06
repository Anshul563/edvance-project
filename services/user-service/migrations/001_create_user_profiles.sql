CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Profile state owned by user-service. user_id is the external identity
-- issued by auth-service (JWT sub); there is deliberately NO foreign key
-- because the identity lives in a different database.
CREATE TABLE IF NOT EXISTS user_profiles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    username VARCHAR(30) NOT NULL,
    display_name VARCHAR(100) NOT NULL,

    bio VARCHAR(500),
    avatar_url TEXT,
    cover_url TEXT,
    website_url TEXT,
    location VARCHAR(100),
    country_code CHAR(2),
    timezone VARCHAR(64),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_profiles_user_id_unique UNIQUE (user_id),
    CONSTRAINT user_profiles_username_unique UNIQUE (username)
);

CREATE INDEX IF NOT EXISTS idx_user_profiles_user_id
    ON user_profiles (user_id);

CREATE INDEX IF NOT EXISTS idx_user_profiles_username
    ON user_profiles (username);
