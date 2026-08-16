BEGIN;

ALTER TABLE messages DROP CONSTRAINT messages_position_key;
ALTER TABLE messages
    ALTER COLUMN position DROP DEFAULT,
    ALTER COLUMN turn_id DROP DEFAULT,
    ALTER COLUMN turn_complete DROP DEFAULT;
ALTER SEQUENCE messages_position_seq OWNED BY NONE;
ALTER TABLE messages
    DROP COLUMN turn_complete,
    DROP COLUMN turn_id,
    DROP COLUMN position;
DROP SEQUENCE messages_position_seq;

COMMIT;
