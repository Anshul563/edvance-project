-- Social Service: follows, likes, comments, saves, playlists.
--
-- Ownership lives in the service layer (JWT sub); these tables store
-- foreign user/content UUIDs as plain UUID columns because the owners
-- live in other services' databases, which this service never touches.
-- All idempotency requirements are enforced by UNIQUE constraints so a
-- duplicate or concurrent write can never create a second row.

-- ------------------------------------------------------------------
-- follows: one direction of a user relationship. UNIQUE pair makes a
-- duplicate/concurrent follow a no-op, and the CHECK forbids self-follow
-- even if the service layer is bypassed.
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS follows (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    follower_id   UUID NOT NULL,
    following_id  UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT follows_pair_unique UNIQUE (follower_id, following_id),
    CONSTRAINT follows_no_self CHECK (follower_id <> following_id)
);

CREATE INDEX IF NOT EXISTS idx_follows_follower_id
    ON follows (follower_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_follows_following_id
    ON follows (following_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_follows_created_at
    ON follows (created_at DESC);

-- ------------------------------------------------------------------
-- likes: a user liking a piece of content. content_type/content_id point
-- at another service's content (the UNIQUE triple makes duplicates safe).
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS likes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    content_type  VARCHAR(16) NOT NULL
                  CONSTRAINT likes_content_type_check
                  CHECK (content_type IN ('video', 'short', 'post', 'comment')),
    content_id    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT likes_user_content_unique
        UNIQUE (user_id, content_type, content_id)
);

CREATE INDEX IF NOT EXISTS idx_likes_content
    ON likes (content_type, content_id);
CREATE INDEX IF NOT EXISTS idx_likes_user
    ON likes (user_id, created_at DESC);

-- ------------------------------------------------------------------
-- comments: threaded comments on content. parent_id NULL = top-level,
-- parent_id set = a reply to a top-level comment. Deletion is soft:
-- status becomes 'deleted' and deleted_at is stamped. like_count and
-- reply_count are maintained by this service's transactions only.
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS comments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    content_type  VARCHAR(16) NOT NULL
                  CONSTRAINT comments_content_type_check
                  CHECK (content_type IN ('video', 'short', 'post', 'course', 'lesson')),
    content_id    UUID NOT NULL,
    parent_id     UUID REFERENCES comments (id) ON DELETE SET NULL,
    body          TEXT NOT NULL,
    status        VARCHAR(8) NOT NULL DEFAULT 'active'
                  CONSTRAINT comments_status_check
                  CHECK (status IN ('active', 'hidden', 'deleted')),
    like_count    INTEGER NOT NULL DEFAULT 0,
    reply_count   INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_comments_content
    ON comments (content_type, content_id, parent_id);
CREATE INDEX IF NOT EXISTS idx_comments_parent
    ON comments (parent_id);
CREATE INDEX IF NOT EXISTS idx_comments_user
    ON comments (user_id);
CREATE INDEX IF NOT EXISTS idx_comments_newest
    ON comments (content_type, content_id, created_at DESC)
    WHERE parent_id IS NULL;
CREATE INDEX IF NOT EXISTS idx_comments_popular
    ON comments (content_type, content_id, like_count DESC, created_at DESC)
    WHERE parent_id IS NULL;

-- ------------------------------------------------------------------
-- comment_likes: likes on comments. UNIQUE pair makes dupes safe; the
-- comment.like_count column is bumped transactionally beside these rows.
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS comment_likes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    comment_id  UUID NOT NULL REFERENCES comments (id) ON DELETE CASCADE,
    user_id     UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT comment_likes_pair_unique UNIQUE (comment_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_comment_likes_comment
    ON comment_likes (comment_id);
CREATE INDEX IF NOT EXISTS idx_comment_likes_user
    ON comment_likes (user_id, created_at DESC);

-- ------------------------------------------------------------------
-- saves: bookmarks. UNIQUE triple makes a re-save a no-op.
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS saves (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    content_type  VARCHAR(16) NOT NULL
                  CONSTRAINT saves_content_type_check
                  CHECK (content_type IN ('video', 'short', 'post', 'course')),
    content_id    UUID NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT saves_user_content_unique
        UNIQUE (user_id, content_type, content_id)
);

CREATE INDEX IF NOT EXISTS idx_saves_user
    ON saves (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_saves_content
    ON saves (content_type, content_id);

-- ------------------------------------------------------------------
-- playlists: user-curated collections. Visibility governs who may read:
-- private = owner only; unlisted/public = anyone with the link.
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS playlists (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL,
    title         VARCHAR(150) NOT NULL,
    description   TEXT,
    visibility    VARCHAR(10) NOT NULL DEFAULT 'private'
                  CONSTRAINT playlists_visibility_check
                  CHECK (visibility IN ('private', 'unlisted', 'public')),
    thumbnail_url TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_playlists_user
    ON playlists (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_playlists_visibility
    ON playlists (visibility);

-- ------------------------------------------------------------------
-- playlist_items: ordered members of a playlist. Two UNIQUE constraints
-- cover "no duplicate content in one playlist" and "no two items share
-- a position". position is 1-based; reorder rewrites it in a transaction.
-- ------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS playlist_items (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    playlist_id   UUID NOT NULL REFERENCES playlists (id) ON DELETE CASCADE,
    content_type  VARCHAR(16) NOT NULL
                  CONSTRAINT playlist_items_content_type_check
                  CHECK (content_type IN ('video', 'short', 'post', 'course')),
    content_id    UUID NOT NULL,
    position      INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT playlist_items_content_unique
        UNIQUE (playlist_id, content_type, content_id),
    CONSTRAINT playlist_items_position_unique
        UNIQUE (playlist_id, position)
);

CREATE INDEX IF NOT EXISTS idx_playlist_items_playlist
    ON playlist_items (playlist_id, position);