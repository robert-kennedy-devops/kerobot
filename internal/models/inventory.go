package models

type Inventory struct {
	ID         int64 `db:"id"`
	TelegramID int64 `db:"telegram_id"`
	Potions    int   `db:"potions"`
}
