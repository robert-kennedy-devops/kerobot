package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"kerobot/internal/accounts"
	"kerobot/internal/config"
	"kerobot/internal/configbot"
	"kerobot/internal/database"
	"kerobot/pkg/logger"
	"kerobot/pkg/metrics"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", slog.Any("err", err))
		os.Exit(1)
	}

	log := logger.New(parseLogLevel(cfg.LogLevel))
	slog.SetDefault(log)
	log.Info("starting KeroBot")
	log.Info("config bot onboarding", slog.String("steps", "1) get API_ID/API_HASH at my.telegram.org -> API development tools; 2) /set_api <id>; 3) /set_hash <hash>; 4) /qr"))

	db, err := database.ConnectWithRetry(ctx, cfg.DSN(), cfg.Database.ConnectAttempts, cfg.Database.ConnectBackoff)
	if err != nil {
		log.Error("db connect failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	if err := database.RunMigrations(ctx, db, "./migrations"); err != nil {
		log.Error("migrations failed", slog.Any("err", err))
		os.Exit(1)
	}

	cfgRepo := database.NewConfigRepo(db.Pool)
	counters := &metrics.Counters{}
	manager := accounts.NewManager(cfg, cfgRepo, log, counters)
	manager.StartExisting(ctx)

	if cfg.Bot.Enabled && cfg.Bot.Token != "" {
		if cfg.Bot.AdminChatID == 0 {
			log.Warn("ADMIN_CHAT_ID not set: config bot is open to ALL users — set ADMIN_CHAT_ID to restrict access")
		}
		bot, err := configbot.New(cfg.Bot.Token, cfg.Bot.AdminChatID, cfgRepo, log, manager)
		if err != nil {
			log.Error("config bot init failed", slog.Any("err", err))
		} else {
			go bot.Start(ctx)
			log.Info("config bot started")
		}
	} else {
		log.Warn("config bot disabled or missing token")
	}

	go startMetrics(ctx, log, cfg.Metrics.Addr, counters)

	<-ctx.Done()
	log.Info("shutdown")
}

func startMetrics(ctx context.Context, log *slog.Logger, addr string, counters *metrics.Counters) {
	if addr == "" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(counters.Snapshot())
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(ctxShutdown)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("metrics server error", slog.Any("err", err))
	}
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
