-- reasoning persists an assistant round's accumulated thinking text (streamed
-- live over SSE but previously discarded once the round closed, so a page
-- reload lost every thinking block). is_error persists a tool-result row's
-- error flag (previously only sent live over SSE) so a reloaded tool card
-- renders the same failed/succeeded state it had live.
ALTER TABLE messages ADD COLUMN reasoning TEXT NOT NULL DEFAULT '';
ALTER TABLE messages ADD COLUMN is_error  BOOLEAN NOT NULL DEFAULT false;
