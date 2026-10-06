CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Creator identity owned by creator-service. user_id is the external
-- identity issued by auth-service (JWT sub); there is deliberately NO
-- foreign key because the identity lives in a different database.
-- Records are never physically deleted: status transitions
-- (active/suspended/disabled) preserve course, video, analytics, and
-- monetization history.
CREATE TABLE IF NOT EXISTS creators (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'active',

    display_name VARCHAR(100) NOT NULL,
    headline VARCHAR(150),
    bio VARCHAR(1000),
    avatar_url TEXT,
    cover_url TEXT,
    website_url TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT creators_user_id_unique UNIQUE (user_id),

    CONSTRAINT creators_status_check
        CHECK (status IN ('pending', 'active', 'suspended', 'disabled'))
);

-- One public channel per creator (v1). handle is the public URL slug.
CREATE TABLE IF NOT EXISTS creator_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    creator_id UUID NOT NULL,

    handle VARCHAR(30) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(1000),
    banner_url TEXT,
    avatar_url TEXT,

    status VARCHAR(20) NOT NULL DEFAULT 'active',

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT creator_channels_creator_id_unique UNIQUE (creator_id),
    CONSTRAINT creator_channels_handle_unique UNIQUE (handle),

    CONSTRAINT creator_channels_status_check
        CHECK (status IN ('active', 'hidden'))
);

CREATE INDEX IF NOT EXISTS idx_creators_user_id
    ON creators (user_id);

CREATE INDEX IF NOT EXISTS idx_creator_channels_creator_id
    ON creator_channels (creator_id);

CREATE INDEX IF NOT EXISTS idx_creator_channels_handle
    ON creator_channels (handle);
