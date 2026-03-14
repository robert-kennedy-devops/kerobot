package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"kerobot/internal/models"
)

type ConfigRepo struct {
	db *pgxpool.Pool
}

func NewConfigRepo(db *pgxpool.Pool) *ConfigRepo {
	return &ConfigRepo{db: db}
}

func (r *ConfigRepo) EnsureUserAndConfig(ctx context.Context, telegramID int64) error {
	if telegramID == 0 {
		return nil
	}
	if _, err := r.db.Exec(ctx, `
        INSERT INTO users (telegram_id)
        VALUES ($1)
        ON CONFLICT (telegram_id) DO NOTHING
    `, telegramID); err != nil {
		return fmt.Errorf("ensure user: %w", err)
	}
	if _, err := r.db.Exec(ctx, `
        INSERT INTO configs (telegram_id)
        VALUES ($1)
        ON CONFLICT (telegram_id) DO NOTHING
    `, telegramID); err != nil {
		return fmt.Errorf("ensure config: %w", err)
	}
	return nil
}

func (r *ConfigRepo) GetConfig(ctx context.Context, telegramID int64) (models.Config, error) {
	var cfg models.Config
	row := r.db.QueryRow(ctx, `
        SELECT id, telegram_id, auto_hunt, auto_combat, auto_heal, heal_percent, auto_buy_potions, min_potions, auto_dungeon, capture_enabled
        FROM configs WHERE telegram_id = $1
    `, telegramID)

	if err := row.Scan(&cfg.ID, &cfg.TelegramID, &cfg.AutoHunt, &cfg.AutoCombat, &cfg.AutoHeal, &cfg.HealPercent, &cfg.AutoBuyPotions, &cfg.MinPotions, &cfg.AutoDungeon, &cfg.CaptureEnabled); err != nil {
		if err == pgx.ErrNoRows {
			if err := r.EnsureUserAndConfig(ctx, telegramID); err != nil {
				return cfg, err
			}
			row = r.db.QueryRow(ctx, `
                SELECT id, telegram_id, auto_hunt, auto_combat, auto_heal, heal_percent, auto_buy_potions, min_potions, auto_dungeon, capture_enabled
                FROM configs WHERE telegram_id = $1
            `, telegramID)
			if err := row.Scan(&cfg.ID, &cfg.TelegramID, &cfg.AutoHunt, &cfg.AutoCombat, &cfg.AutoHeal, &cfg.HealPercent, &cfg.AutoBuyPotions, &cfg.MinPotions, &cfg.AutoDungeon, &cfg.CaptureEnabled); err != nil {
				return cfg, fmt.Errorf("read config: %w", err)
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("read config: %w", err)
	}

	return cfg, nil
}

func (r *ConfigRepo) UpdateToggle(ctx context.Context, telegramID int64, field string, value bool) error {
	allowed := map[string]bool{
		"auto_hunt":        true,
		"auto_combat":      true,
		"auto_heal":        true,
		"auto_buy_potions": true,
		"auto_dungeon":     true,
		"capture_enabled":  true,
	}
	if !allowed[field] {
		return fmt.Errorf("invalid field: %s", field)
	}
	query := fmt.Sprintf(`UPDATE configs SET %s = $1 WHERE telegram_id = $2`, field)
	_, err := r.db.Exec(ctx, query, value, telegramID)
	return err
}

func (r *ConfigRepo) UpdateHealPercent(ctx context.Context, telegramID int64, percent int) error {
	_, err := r.db.Exec(ctx, `UPDATE configs SET heal_percent = $1 WHERE telegram_id = $2`, percent, telegramID)
	return err
}

func (r *ConfigRepo) UpdateMinPotions(ctx context.Context, telegramID int64, min int) error {
	_, err := r.db.Exec(ctx, `UPDATE configs SET min_potions = $1 WHERE telegram_id = $2`, min, telegramID)
	return err
}

func (r *ConfigRepo) SaveCapture(ctx context.Context, c models.Capture) error {
	buttons, err := json.Marshal(c.Buttons)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
        INSERT INTO captures (telegram_id, state, text, buttons, hp_percent, potions)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, c.TelegramID, c.State, c.Text, buttons, c.HPPercent, c.Potions)
	return err
}

func (r *ConfigRepo) LastCapture(ctx context.Context, telegramID int64) (models.Capture, error) {
	var c models.Capture
	var buttonsJSON []byte
	row := r.db.QueryRow(ctx, `
        SELECT id, telegram_id, state, text, buttons, hp_percent, potions, created_at
        FROM captures WHERE telegram_id = $1
        ORDER BY id DESC LIMIT 1
    `, telegramID)
	if err := row.Scan(&c.ID, &c.TelegramID, &c.State, &c.Text, &buttonsJSON, &c.HPPercent, &c.Potions, &c.CreatedAt); err != nil {
		return c, err
	}
	if len(buttonsJSON) > 0 {
		_ = json.Unmarshal(buttonsJSON, &c.Buttons)
	}
	return c, nil
}

func (r *ConfigRepo) SaveRule(ctx context.Context, rule models.Rule) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO rules (telegram_id, match_type, match_value, action_type, action_value, enabled)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, rule.TelegramID, rule.MatchType, rule.MatchValue, rule.ActionType, rule.ActionValue, rule.Enabled)
	return err
}

func (r *ConfigRepo) ListRules(ctx context.Context, telegramID int64) ([]models.Rule, error) {
	rows, err := r.db.Query(ctx, `
        SELECT id, telegram_id, match_type, match_value, action_type, action_value, enabled, created_at
        FROM rules WHERE telegram_id = $1 ORDER BY id DESC
    `, telegramID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Rule
	for rows.Next() {
		var r0 models.Rule
		if err := rows.Scan(&r0.ID, &r0.TelegramID, &r0.MatchType, &r0.MatchValue, &r0.ActionType, &r0.ActionValue, &r0.Enabled, &r0.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r0)
	}
	return out, rows.Err()
}

func (r *ConfigRepo) GetAppConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRow(ctx, `SELECT value FROM app_config WHERE key = $1`, key).Scan(&value)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *ConfigRepo) SetAppConfig(ctx context.Context, key, value string) error {
	_, err := r.db.Exec(ctx, `
        INSERT INTO app_config (key, value)
        VALUES ($1, $2)
        ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
    `, key, value)
	return err
}
