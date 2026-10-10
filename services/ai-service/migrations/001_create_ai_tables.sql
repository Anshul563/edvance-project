CREATE TABLE IF NOT EXISTS ai_conversations (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    course_id VARCHAR(36),
    title VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_conversations_user_id ON ai_conversations (user_id);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_course_id ON ai_conversations (course_id);
CREATE INDEX IF NOT EXISTS idx_ai_conversations_created_at ON ai_conversations (created_at);

CREATE TABLE IF NOT EXISTS ai_messages (
    id VARCHAR(36) PRIMARY KEY,
    conversation_id VARCHAR(36) NOT NULL REFERENCES ai_conversations(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL,
    content TEXT NOT NULL,
    source_references JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_messages_conversation_id ON ai_messages (conversation_id);
CREATE INDEX IF NOT EXISTS idx_ai_messages_role ON ai_messages (role);

CREATE TABLE IF NOT EXISTS ai_usage_records (
    id VARCHAR(36) PRIMARY KEY,
    user_id VARCHAR(36) NOT NULL,
    conversation_id VARCHAR(36),
    request_id VARCHAR(36),
    provider VARCHAR(64) NOT NULL,
    model VARCHAR(128) NOT NULL,
    operation VARCHAR(64) NOT NULL,
    input_tokens INTEGER,
    output_tokens INTEGER,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_records_user_id ON ai_usage_records (user_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_conversation_id ON ai_usage_records (conversation_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_operation ON ai_usage_records (operation);
CREATE INDEX IF NOT EXISTS idx_ai_usage_records_status ON ai_usage_records (status);
