DROP TABLE IF EXISTS bot_message_targets;

ALTER TABLE bot_users
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS last_name,
    DROP COLUMN IF EXISTS first_name,
    DROP COLUMN IF EXISTS username;
