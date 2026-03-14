package models

import "time"

type Dungeon struct {
	ID        int64     `db:"id"`
	UserID    int64     `db:"user_id"`
	Status    string    `db:"status"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}
