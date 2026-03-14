package models

type Config struct {
	ID             int64 `db:"id"`
	TelegramID     int64 `db:"telegram_id"`
	AutoHunt       bool  `db:"auto_hunt"`
	AutoCombat     bool  `db:"auto_combat"`
	AutoHeal       bool  `db:"auto_heal"`
	HealPercent    int   `db:"heal_percent"`
	AutoBuyPotions bool  `db:"auto_buy_potions"`
	MinPotions     int   `db:"min_potions"`
	AutoDungeon    bool  `db:"auto_dungeon"`
	CaptureEnabled bool  `db:"capture_enabled"`
}
