CREATE TABLE IF NOT EXISTS contests (
    id SERIAL PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('draft', 'active', 'closed')),
    reward_pack_id INT NOT NULL REFERENCES sticker_packs(id),
    winner_user_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS contest_channels (
    contest_id INT NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    channel_id BIGINT NOT NULL,
    channel_link TEXT NOT NULL,
    position SMALLINT NOT NULL,
    PRIMARY KEY (contest_id, channel_id),
    UNIQUE (contest_id, position)
);

CREATE TABLE IF NOT EXISTS contest_participants (
    contest_id INT NOT NULL REFERENCES contests(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (contest_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_contest_participants_contest_id
    ON contest_participants (contest_id);
