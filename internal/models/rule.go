package models

import "time"

type Rule struct {
	ID          int64     `db:"id"`
	TelegramID  int64     `db:"telegram_id"`
	MatchType   string    `db:"match_type"`
	MatchValue  string    `db:"match_value"`
	ActionType  string    `db:"action_type"`
	ActionValue string    `db:"action_value"`
	Enabled     bool      `db:"enabled"`
	CreatedAt   time.Time `db:"created_at"`
}
