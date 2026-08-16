CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at    DATETIME,
    username      TEXT NOT NULL,
    password_hash TEXT,
    is_admin      BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE UNIQUE INDEX idx_users_username ON users (username) WHERE deleted_at IS NULL;
CREATE INDEX idx_users_deleted_at ON users (deleted_at);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    user_id    TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL
);

CREATE UNIQUE INDEX idx_sessions_token_hash ON sessions (token_hash) WHERE deleted_at IS NULL;
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
CREATE INDEX idx_sessions_expires_at ON sessions (expires_at);
CREATE INDEX idx_sessions_deleted_at ON sessions (deleted_at);

CREATE TABLE chats (
    id                      TEXT PRIMARY KEY,
    created_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at              DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at              DATETIME,
    user_id                 TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title                   TEXT NOT NULL DEFAULT '',
    model_id                TEXT NOT NULL DEFAULT '',
    temperature             REAL,
    top_p                   REAL,
    reasoning_effort        TEXT NOT NULL DEFAULT '',
    max_output_tokens       INTEGER,
    max_history_tokens      INTEGER,
    disabled_mcp_server_ids JSON NOT NULL DEFAULT '[]'
);

CREATE INDEX idx_chats_user_id ON chats (user_id);
CREATE INDEX idx_chats_deleted_at ON chats (deleted_at);

CREATE TABLE message_positions (
    id       INTEGER PRIMARY KEY CHECK (id = 1),
    position INTEGER NOT NULL
);

INSERT INTO message_positions (id, position) VALUES (1, 0);

CREATE TABLE messages (
    id                 TEXT PRIMARY KEY,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at         DATETIME,
    position           INTEGER NOT NULL DEFAULT 0 UNIQUE,
    turn_id            TEXT NOT NULL,
    turn_complete      BOOLEAN NOT NULL DEFAULT TRUE,
    chat_id            TEXT NOT NULL REFERENCES chats (id) ON DELETE CASCADE,
    role               TEXT NOT NULL,
    content            TEXT NOT NULL DEFAULT '',
    reasoning          TEXT NOT NULL DEFAULT '',
    provider_reasoning JSON,
    tool_calls         JSON,
    tool_call_id       TEXT NOT NULL DEFAULT '',
    is_error           BOOLEAN NOT NULL DEFAULT FALSE,
    is_injection       BOOLEAN NOT NULL DEFAULT FALSE,
    ui_spec            JSON,
    input_tokens       INTEGER NOT NULL DEFAULT 0,
    output_tokens      INTEGER NOT NULL DEFAULT 0
);

CREATE TRIGGER messages_assign_position
AFTER INSERT ON messages
FOR EACH ROW WHEN NEW.position = 0
BEGIN
    UPDATE message_positions SET position = position + 1 WHERE id = 1;
    UPDATE messages
    SET position = (SELECT position FROM message_positions WHERE id = 1)
    WHERE rowid = NEW.rowid;
END;

CREATE INDEX idx_messages_chat_id ON messages (chat_id);
CREATE INDEX idx_messages_deleted_at ON messages (deleted_at);

CREATE TABLE llm_usage (
    id                   TEXT PRIMARY KEY,
    created_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at           DATETIME,
    service              TEXT NOT NULL DEFAULT '',
    stage                TEXT NOT NULL DEFAULT '',
    model                TEXT NOT NULL DEFAULT '',
    reasoning_effort     TEXT NOT NULL DEFAULT '',
    prompt_tokens        INTEGER NOT NULL DEFAULT 0,
    cached_prompt_tokens INTEGER NOT NULL DEFAULT 0,
    completion_tokens    INTEGER NOT NULL DEFAULT 0,
    reasoning_tokens     INTEGER NOT NULL DEFAULT 0,
    total_tokens         INTEGER NOT NULL DEFAULT 0,
    duration_ms          INTEGER NOT NULL DEFAULT 0,
    user_id              TEXT,
    chat_id              TEXT,
    message_id           TEXT,
    request_id           TEXT NOT NULL DEFAULT '',
    error_message        TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_llm_usage_user_id ON llm_usage (user_id);
CREATE INDEX idx_llm_usage_chat_id ON llm_usage (chat_id);
CREATE INDEX idx_llm_usage_message_id ON llm_usage (message_id);
CREATE INDEX idx_llm_usage_request_id ON llm_usage (request_id);
CREATE INDEX idx_llm_usage_created_at ON llm_usage (created_at);

CREATE TABLE mcp_servers (
    id          TEXT PRIMARY KEY,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  DATETIME,
    name        TEXT NOT NULL,
    transport   TEXT NOT NULL,
    command     TEXT NOT NULL DEFAULT '',
    args        JSON,
    url         TEXT NOT NULL DEFAULT '',
    headers_enc BLOB,
    env_enc     BLOB,
    enabled     BOOLEAN NOT NULL DEFAULT TRUE,
    created_by  TEXT
);

CREATE UNIQUE INDEX idx_mcp_servers_name ON mcp_servers (name) WHERE deleted_at IS NULL;
CREATE INDEX idx_mcp_servers_deleted_at ON mcp_servers (deleted_at);

CREATE TABLE mcp_tool_executions (
    id          TEXT PRIMARY KEY,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  DATETIME,
    message_id  TEXT NOT NULL REFERENCES messages (id) ON DELETE CASCADE,
    server      TEXT NOT NULL,
    tool        TEXT NOT NULL,
    params      JSON,
    result      TEXT NOT NULL DEFAULT '',
    is_error    BOOLEAN NOT NULL DEFAULT FALSE,
    duration_ms INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_mcp_tool_executions_message_id ON mcp_tool_executions (message_id);
CREATE INDEX idx_mcp_tool_executions_deleted_at ON mcp_tool_executions (deleted_at);
