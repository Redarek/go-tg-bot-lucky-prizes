ALTER TABLE bot_users
    ADD COLUMN IF NOT EXISTS username TEXT,
    ADD COLUMN IF NOT EXISTS first_name TEXT,
    ADD COLUMN IF NOT EXISTS last_name TEXT,
    ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now();

CREATE TABLE IF NOT EXISTS bot_message_targets (
    user_id BIGINT PRIMARY KEY REFERENCES bot_users(user_id) ON DELETE CASCADE,
    can_message BOOLEAN NOT NULL DEFAULT TRUE,
    last_confirmed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error_code INTEGER,
    last_error_text TEXT,
    blocked_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO bot_message_targets (user_id, can_message)
SELECT user_id, TRUE
FROM bot_users
ON CONFLICT (user_id) DO NOTHING;
