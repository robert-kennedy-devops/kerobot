CREATE TABLE IF NOT EXISTS rules (
    id BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT NOT NULL,
    match_type TEXT NOT NULL,
    match_value TEXT NOT NULL,
    action_type TEXT NOT NULL,
    action_value TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rules_telegram_id ON rules (telegram_id);
