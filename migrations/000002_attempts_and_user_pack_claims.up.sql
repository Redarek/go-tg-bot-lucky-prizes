CREATE TABLE IF NOT EXISTS user_attempts (
    user_id BIGINT PRIMARY KEY,
    attempts INT NOT NULL DEFAULT 1 CHECK (attempts >= 0)
);

CREATE TABLE IF NOT EXISTS user_pack_claims (
    user_id BIGINT NOT NULL,
    pack_id INT NOT NULL REFERENCES sticker_packs(id) ON DELETE CASCADE,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, pack_id)
);

INSERT INTO user_attempts (user_id, attempts)
SELECT bu.user_id,
       CASE WHEN uc.user_id IS NOT NULL THEN 0 ELSE 1 END
FROM bot_users bu
LEFT JOIN user_claims uc ON uc.user_id = bu.user_id
ON CONFLICT (user_id) DO NOTHING;
