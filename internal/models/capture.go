package models

import "time"

type Capture struct {
	ID         int64     `db:"id"`
	TelegramID int64     `db:"telegram_id"`
	State      string    `db:"state"`
	Text       string    `db:"text"`
	Buttons    []string  `db:"buttons"`
	HPPercent  int       `db:"hp_percent"`
	Potions    int       `db:"potions"`
	CreatedAt  time.Time `db:"created_at"`
}
