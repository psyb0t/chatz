BEGIN;

ALTER TABLE messages
    ADD COLUMN position BIGINT,
    ADD COLUMN turn_id UUID,
    ADD COLUMN turn_complete BOOLEAN;

CREATE SEQUENCE messages_position_seq AS BIGINT;
ALTER SEQUENCE messages_position_seq OWNED BY messages.position;

WITH ordered_messages AS MATERIALIZED (
    SELECT
        id,
        chat_id,
        role,
        ROW_NUMBER() OVER (ORDER BY created_at, id) AS position,
        COUNT(*) FILTER (WHERE role = 'user') OVER (
            PARTITION BY chat_id
            ORDER BY created_at, id
            ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
        ) AS turn_number
    FROM messages
), inferred_turns AS MATERIALIZED (
    SELECT
        chat_id,
        turn_number,
        gen_random_uuid() AS turn_id,
        (ARRAY_AGG(role ORDER BY position DESC))[1] = 'assistant' AS turn_complete
    FROM ordered_messages
    GROUP BY chat_id, turn_number
)
UPDATE messages
SET
    position = ordered_messages.position,
    turn_id = inferred_turns.turn_id,
    turn_complete = inferred_turns.turn_complete
FROM ordered_messages
JOIN inferred_turns USING (chat_id, turn_number)
WHERE messages.id = ordered_messages.id;

SELECT setval(
    'messages_position_seq',
    COALESCE((SELECT MAX(position) FROM messages), 0) + 1,
    false
);

ALTER TABLE messages
    ALTER COLUMN position SET DEFAULT nextval('messages_position_seq'),
    ALTER COLUMN position SET NOT NULL,
    ALTER COLUMN turn_id SET DEFAULT gen_random_uuid(),
    ALTER COLUMN turn_id SET NOT NULL,
    ALTER COLUMN turn_complete SET DEFAULT true,
    ALTER COLUMN turn_complete SET NOT NULL;

ALTER TABLE messages
    ADD CONSTRAINT messages_position_key UNIQUE (position);

COMMIT;
