CREATE TABLE llm_usage (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ,
    service              TEXT NOT NULL DEFAULT '',
    stage                TEXT NOT NULL DEFAULT '',
    model                TEXT NOT NULL DEFAULT '',
    reasoning_effort     TEXT NOT NULL DEFAULT '',
    prompt_tokens        BIGINT NOT NULL DEFAULT 0,
    cached_prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens    BIGINT NOT NULL DEFAULT 0,
    reasoning_tokens     BIGINT NOT NULL DEFAULT 0,
    total_tokens         BIGINT NOT NULL DEFAULT 0,
    duration_ms          BIGINT NOT NULL DEFAULT 0,
    user_id              UUID,
    chat_id              UUID,
    message_id           UUID,
    request_id           TEXT NOT NULL DEFAULT '',
    error_message        TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_llm_usage_user_id ON llm_usage (user_id);
CREATE INDEX idx_llm_usage_chat_id ON llm_usage (chat_id);
CREATE INDEX idx_llm_usage_message_id ON llm_usage (message_id);
CREATE INDEX idx_llm_usage_request_id ON llm_usage (request_id);
CREATE INDEX idx_llm_usage_created_at ON llm_usage (created_at);
