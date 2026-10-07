CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Student learning state owned by learning-service. user_id, course_id,
-- and lesson_id are cross-service identifiers (plain UUIDs): deliberately
-- NO foreign keys outside this database. lesson_progress.enrollment_id
-- and learning_activities.enrollment_id ARE relationally connected here
-- (same database) and cascade with their enrollment.
CREATE TABLE IF NOT EXISTS enrollments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    course_id UUID NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'active',

    source VARCHAR(20) NOT NULL DEFAULT 'free',

    enrolled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    started_at TIMESTAMPTZ,

    completed_at TIMESTAMPTZ,

    last_accessed_at TIMESTAMPTZ,

    last_lesson_id UUID,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT enrollments_status_check
        CHECK (
            status IN (
                'active',
                'completed',
                'cancelled',
                'suspended'
            )
        ),

    CONSTRAINT enrollments_source_check
        CHECK (
            source IN (
                'free',
                'manual'
            )
        ),

    CONSTRAINT enrollments_user_course_unique
        UNIQUE (user_id, course_id)
);

CREATE TABLE IF NOT EXISTS lesson_progress (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    enrollment_id UUID NOT NULL REFERENCES enrollments (id) ON DELETE CASCADE,

    lesson_id UUID NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'not_started',

    progress_percent INTEGER NOT NULL DEFAULT 0,

    watched_seconds BIGINT NOT NULL DEFAULT 0,

    last_position_seconds BIGINT NOT NULL DEFAULT 0,

    started_at TIMESTAMPTZ,

    completed_at TIMESTAMPTZ,

    last_accessed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT lesson_progress_status_check
        CHECK (
            status IN (
                'not_started',
                'in_progress',
                'completed'
            )
        ),

    CONSTRAINT lesson_progress_percent_check
        CHECK (
            progress_percent >= 0
            AND progress_percent <= 100
        ),

    CONSTRAINT lesson_progress_watched_check
        CHECK (watched_seconds >= 0),

    CONSTRAINT lesson_progress_position_check
        CHECK (last_position_seconds >= 0),

    CONSTRAINT lesson_progress_unique
        UNIQUE (enrollment_id, lesson_id)
);

CREATE TABLE IF NOT EXISTS learning_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    user_id UUID NOT NULL,

    enrollment_id UUID NOT NULL REFERENCES enrollments (id) ON DELETE CASCADE,

    lesson_id UUID,

    activity_type VARCHAR(30) NOT NULL,

    metadata JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT learning_activities_type_check
        CHECK (
            activity_type IN (
                'enrolled',
                'lesson_started',
                'lesson_progressed',
                'lesson_completed',
                'course_completed'
            )
        )
);

CREATE INDEX IF NOT EXISTS idx_enrollments_user_id
    ON enrollments(user_id);

CREATE INDEX IF NOT EXISTS idx_enrollments_course_id
    ON enrollments(course_id);

CREATE INDEX IF NOT EXISTS idx_enrollments_status
    ON enrollments(status);

CREATE INDEX IF NOT EXISTS idx_enrollments_last_accessed_at
    ON enrollments(last_accessed_at);

CREATE INDEX IF NOT EXISTS idx_lesson_progress_enrollment_id
    ON lesson_progress(enrollment_id);

CREATE INDEX IF NOT EXISTS idx_lesson_progress_lesson_id
    ON lesson_progress(lesson_id);

CREATE INDEX IF NOT EXISTS idx_lesson_progress_status
    ON lesson_progress(status);

CREATE INDEX IF NOT EXISTS idx_lesson_progress_last_accessed
    ON lesson_progress(last_accessed_at);

CREATE INDEX IF NOT EXISTS idx_learning_activities_user_id
    ON learning_activities(user_id);

CREATE INDEX IF NOT EXISTS idx_learning_activities_enrollment_id
    ON learning_activities(enrollment_id);

CREATE INDEX IF NOT EXISTS idx_learning_activities_created_at
    ON learning_activities(created_at);
