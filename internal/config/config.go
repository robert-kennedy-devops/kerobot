package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Telegram   TelegramConfig
	Database   DatabaseConfig
	Automation AutomationConfig
	Limits     LimitsConfig
	Metrics    MetricsConfig
	LogLevel   string
	Bot        BotConfig
}

type TelegramConfig struct {
	APIID       int
	APIHash     string
	Password    string
	SessionPath string
	SessionDir  string
	TargetBot   string
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Pass            string
	Name            string
	SSLMode         string
	ConnectAttempts int
	ConnectBackoff  time.Duration
}

type AutomationConfig struct {
	HuntInterval    time.Duration
	CombatInterval  time.Duration
	HealInterval    time.Duration
	PotionInterval  time.Duration
	DungeonInterval time.Duration
	HealPercent     int
	MinPotions      int
}

type LimitsConfig struct {
	ClickDelay    time.Duration
	RatePerSecond int
	RetryAttempts int
	RetryDelay    time.Duration
}

type MetricsConfig struct {
	Addr string
}

type BotConfig struct {
	Token       string
	AdminChatID int64
	Enabled     bool
}

func Load() (Config, error) {
	cfg := Config{
		Telegram: TelegramConfig{
			SessionPath: "./data/telegram.session",
			TargetBot:   "Teletofusbot",
			SessionDir:  "./data/sessions",
		},
		Database: DatabaseConfig{
			Host:            "postgres",
			Port:            5432,
			SSLMode:         "disable",
			ConnectAttempts: 10,
			ConnectBackoff:  2 * time.Second,
		},
		Automation: AutomationConfig{
			HuntInterval:    15 * time.Second,
			CombatInterval:  5 * time.Second,
			HealInterval:    10 * time.Second,
			PotionInterval:  30 * time.Second,
			DungeonInterval: 1 * time.Minute,
			HealPercent:     40,
			MinPotions:      5,
		},
		Limits: LimitsConfig{
			ClickDelay:    900 * time.Millisecond,
			RatePerSecond: 2,
			RetryAttempts: 3,
			RetryDelay:    1500 * time.Millisecond,
		},
		Metrics: MetricsConfig{
			Addr: ":9090",
		},
		LogLevel: "info",
		Bot: BotConfig{
			Enabled: true,
		},
	}

	cfg.Telegram.APIID = envInt("API_ID", 0)
	cfg.Telegram.APIHash = envString("API_HASH", "")
	cfg.Telegram.Password = envString("TG_PASSWORD", "")
	cfg.Telegram.SessionPath = envString("TG_SESSION", cfg.Telegram.SessionPath)
	cfg.Telegram.SessionDir = envString("TG_SESSION_DIR", cfg.Telegram.SessionDir)
	cfg.Telegram.TargetBot = envString("TARGET_BOT", cfg.Telegram.TargetBot)

	cfg.Database.Host = envString("DB_HOST", cfg.Database.Host)
	cfg.Database.Port = envInt("DB_PORT", cfg.Database.Port)
	cfg.Database.User = envString("DB_USER", "kerobot")
	cfg.Database.Pass = envString("DB_PASS", "kerobot")
	cfg.Database.Name = envString("DB_NAME", "kerobot")
	cfg.Database.SSLMode = envString("DB_SSLMODE", cfg.Database.SSLMode)
	cfg.Database.ConnectAttempts = envInt("DB_CONNECT_ATTEMPTS", cfg.Database.ConnectAttempts)
	cfg.Database.ConnectBackoff = envDuration("DB_CONNECT_BACKOFF", cfg.Database.ConnectBackoff)

	cfg.Automation.HuntInterval = envDuration("HUNT_INTERVAL", cfg.Automation.HuntInterval)
	cfg.Automation.CombatInterval = envDuration("COMBAT_INTERVAL", cfg.Automation.CombatInterval)
	cfg.Automation.HealInterval = envDuration("HEAL_INTERVAL", cfg.Automation.HealInterval)
	cfg.Automation.PotionInterval = envDuration("POTION_INTERVAL", cfg.Automation.PotionInterval)
	cfg.Automation.DungeonInterval = envDuration("DUNGEON_INTERVAL", cfg.Automation.DungeonInterval)
	cfg.Automation.HealPercent = envInt("HEAL_PERCENT", cfg.Automation.HealPercent)
	cfg.Automation.MinPotions = envInt("MIN_POTIONS", cfg.Automation.MinPotions)

	cfg.Limits.ClickDelay = envDuration("CLICK_DELAY", cfg.Limits.ClickDelay)
	cfg.Limits.RatePerSecond = envInt("RATE_PER_SECOND", cfg.Limits.RatePerSecond)
	cfg.Limits.RetryAttempts = envInt("RETRY_ATTEMPTS", cfg.Limits.RetryAttempts)
	cfg.Limits.RetryDelay = envDuration("RETRY_DELAY", cfg.Limits.RetryDelay)

	cfg.Metrics.Addr = envString("METRICS_ADDR", cfg.Metrics.Addr)
	cfg.LogLevel = envString("LOG_LEVEL", cfg.LogLevel)
	cfg.Bot.Token = envString("BOT_TOKEN", "")
	cfg.Bot.AdminChatID = envInt64("ADMIN_CHAT_ID", 0)
	cfg.Bot.Enabled = envBool("BOT_ENABLED", cfg.Bot.Enabled)

	if cfg.Database.Host == "" || cfg.Database.User == "" || cfg.Database.Name == "" {
		return cfg, errors.New("missing database configuration")
	}

	return cfg, nil
}

func (c Config) DSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", c.Database.User, c.Database.Pass, c.Database.Host, c.Database.Port, c.Database.Name, c.Database.SSLMode)
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func envInt64(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return i
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y", "on":
			return true
		case "0", "false", "no", "n", "off":
			return false
		}
	}
	return def
}
