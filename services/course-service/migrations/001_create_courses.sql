CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Educational course structure owned by course-service.
-- creator_id and lesson content_id values are cross-service identifiers
-- (plain UUIDs): deliberately NO foreign keys to other services'
-- databases. Objectives, requirements, sections, and lessons belong to
-- this database, so they ARE relationally connected here.
CREATE TABLE IF NOT EXISTS courses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    creator_id UUID NOT NULL,

    title VARCHAR(200) NOT NULL,
    slug VARCHAR(220) NOT NULL,
    subtitle VARCHAR(300),

    description TEXT,

    thumbnail_url TEXT,

    level VARCHAR(20) NOT NULL DEFAULT 'all_levels',

    language VARCHAR(20) NOT NULL DEFAULT 'en',

    status VARCHAR(20) NOT NULL DEFAULT 'draft',

    visibility VARCHAR(20) NOT NULL DEFAULT 'public',

    price_cents BIGINT NOT NULL DEFAULT 0,

    currency VARCHAR(3) NOT NULL DEFAULT 'INR',

    published_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT courses_slug_unique
        UNIQUE (slug),

    CONSTRAINT courses_level_check
        CHECK (
            level IN (
                'beginner',
                'intermediate',
                'advanced',
                'all_levels'
            )
        ),

    CONSTRAINT courses_status_check
        CHECK (
            status IN (
                'draft',
                'published',
                'archived'
            )
        ),

    CONSTRAINT courses_visibility_check
        CHECK (
            visibility IN (
                'public',
                'private',
                'unlisted'
            )
        ),

    CONSTRAINT courses_price_check
        CHECK (price_cents >= 0)
);

CREATE TABLE IF NOT EXISTS course_learning_objectives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,

    objective TEXT NOT NULL,

    position INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT course_learning_objectives_position_check
        CHECK (position >= 0)
);

CREATE TABLE IF NOT EXISTS course_requirements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,

    requirement TEXT NOT NULL,

    position INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT course_requirements_position_check
        CHECK (position >= 0)
);

CREATE TABLE IF NOT EXISTS course_sections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    course_id UUID NOT NULL REFERENCES courses (id) ON DELETE CASCADE,

    title VARCHAR(200) NOT NULL,

    description TEXT,

    position INTEGER NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT course_sections_position_check
        CHECK (position >= 0)
);

CREATE TABLE IF NOT EXISTS course_lessons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    section_id UUID NOT NULL REFERENCES course_sections (id) ON DELETE CASCADE,

    title VARCHAR(200) NOT NULL,

    description TEXT,

    type VARCHAR(30) NOT NULL,

    content_id UUID,

    position INTEGER NOT NULL,

    is_preview BOOLEAN NOT NULL DEFAULT FALSE,

    duration_seconds BIGINT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT course_lessons_type_check
        CHECK (
            type IN (
                'video',
                'article',
                'quiz',
                'assignment',
                'coding',
                'resource',
                'live'
            )
        ),

    CONSTRAINT course_lessons_position_check
        CHECK (position >= 0),

    CONSTRAINT course_lessons_duration_check
        CHECK (
            duration_seconds IS NULL
            OR duration_seconds >= 0
        )
);

CREATE INDEX IF NOT EXISTS idx_courses_creator_id
    ON courses(creator_id);

CREATE INDEX IF NOT EXISTS idx_courses_status
    ON courses(status);

CREATE INDEX IF NOT EXISTS idx_courses_visibility
    ON courses(visibility);

CREATE INDEX IF NOT EXISTS idx_courses_created_at
    ON courses(created_at);

CREATE INDEX IF NOT EXISTS idx_course_objectives_course_id
    ON course_learning_objectives(course_id);

CREATE INDEX IF NOT EXISTS idx_course_requirements_course_id
    ON course_requirements(course_id);

CREATE INDEX IF NOT EXISTS idx_course_sections_course_id
    ON course_sections(course_id);

CREATE INDEX IF NOT EXISTS idx_course_sections_position
    ON course_sections(course_id, position);

CREATE INDEX IF NOT EXISTS idx_course_lessons_section_id
    ON course_lessons(section_id);

CREATE INDEX IF NOT EXISTS idx_course_lessons_position
    ON course_lessons(section_id, position);

CREATE INDEX IF NOT EXISTS idx_course_lessons_content_id
    ON course_lessons(content_id);
