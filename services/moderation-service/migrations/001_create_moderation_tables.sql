CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS moderation_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(128) NOT NULL,
    version VARCHAR(32) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    thresholds JSONB NOT NULL DEFAULT '{}',
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS moderation_reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_user_id UUID NOT NULL,
    target_type VARCHAR(64) NOT NULL,
    target_id UUID NOT NULL,
    reason VARCHAR(128) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS moderation_cases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID,
    target_type VARCHAR(64) NOT NULL,
    target_id UUID NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'open',
    priority VARCHAR(16) NOT NULL DEFAULT 'normal',
    assigned_moderator_id UUID,
    automated_severity VARCHAR(16) NOT NULL DEFAULT 'low',
    policy_version VARCHAR(32) NOT NULL DEFAULT 'v1',
    source VARCHAR(32) NOT NULL DEFAULT 'report',
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS moderation_assessments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id UUID NOT NULL REFERENCES moderation_cases(id) ON DELETE CASCADE,
    model VARCHAR(64) NOT NULL DEFAULT 'rules_v1',
    decision VARCHAR(32) NOT NULL,
    severity VARCHAR(16) NOT NULL,
    category_scores JSONB NOT NULL DEFAULT '{}',
    matched_codes JSONB NOT NULL DEFAULT '[]',
    explanation TEXT,
    requires_human_review BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS moderation_actions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id UUID NOT NULL REFERENCES moderation_cases(id) ON DELETE CASCADE,
    target_type VARCHAR(64) NOT NULL,
    target_id UUID NOT NULL,
    action_type VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'scheduled',
    actor_type VARCHAR(32) NOT NULL,
    actor_id UUID NOT NULL,
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS moderation_audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id UUID REFERENCES moderation_cases(id) ON DELETE CASCADE,
    report_id UUID REFERENCES moderation_reports(id) ON DELETE SET NULL,
    actor_id UUID,
    action VARCHAR(128) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS moderation_reviews (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    case_id UUID NOT NULL REFERENCES moderation_cases(id) ON DELETE CASCADE,
    reviewer_id UUID NOT NULL,
    outcome VARCHAR(32) NOT NULL,
    reason TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
