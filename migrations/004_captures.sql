ALTER TABLE configs ADD COLUMN IF NOT EXISTS capture_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS captures (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL,
    state TEXT NOT NULL DEFAULT '',
    text TEXT NOT NULL DEFAULT '',
    buttons JSONB NOT NULL DEFAULT '[]',
    hp_percent INTEGER NOT NULL DEFAULT 0,
    potions INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_captures_telegram_id ON captures (telegram_id);
