-- SQLite dialect of the Postgres 0000021. SQLite adds one column per
-- statement; stop_sequences is a JSON array of strings, empty when unset.
ALTER TABLE chats ADD COLUMN top_k INTEGER;
ALTER TABLE chats ADD COLUMN frequency_penalty REAL;
ALTER TABLE chats ADD COLUMN presence_penalty REAL;
ALTER TABLE chats ADD COLUMN seed INTEGER;
ALTER TABLE chats ADD COLUMN stop_sequences JSON NOT NULL DEFAULT '[]';
ALTER TABLE chats ADD COLUMN system_prompt TEXT;
