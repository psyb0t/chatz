-- Per-chat generation controls beyond temperature, top_p, reasoning effort and
-- the token caps. A NULL column is unset, so the provider default applies.
-- stop_sequences is a JSON array of strings, empty when unset.
ALTER TABLE chats
    ADD COLUMN top_k INTEGER,
    ADD COLUMN frequency_penalty DOUBLE PRECISION,
    ADD COLUMN presence_penalty DOUBLE PRECISION,
    ADD COLUMN seed BIGINT,
    ADD COLUMN stop_sequences JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN system_prompt TEXT;
