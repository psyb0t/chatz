-- Per-chat model-generation settings. All nullable = unset means the provider
-- default (temperature/top_p/reasoning_effort) or no cap (max_*_tokens). No
-- table rewrite; existing chats keep every setting unset.
ALTER TABLE chats ADD COLUMN temperature        DOUBLE PRECISION;
ALTER TABLE chats ADD COLUMN top_p              DOUBLE PRECISION;
ALTER TABLE chats ADD COLUMN reasoning_effort   TEXT NOT NULL DEFAULT '';
ALTER TABLE chats ADD COLUMN max_output_tokens  INTEGER;
ALTER TABLE chats ADD COLUMN max_history_tokens INTEGER;
