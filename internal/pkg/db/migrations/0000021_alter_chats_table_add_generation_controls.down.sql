ALTER TABLE chats
    DROP COLUMN system_prompt,
    DROP COLUMN stop_sequences,
    DROP COLUMN seed,
    DROP COLUMN presence_penalty,
    DROP COLUMN frequency_penalty,
    DROP COLUMN top_k;
