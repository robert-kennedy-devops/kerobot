package automation

import (
	"context"
	"log/slog"
	"time"

	"github.com/gotd/td/tg"

	"kerobot/internal/engine"
	"kerobot/internal/models"
	"kerobot/internal/parser"
)

type ConfigReader interface {
	GetConfig(ctx context.Context, telegramID int64) (models.Config, error)
}

type AutoHuntWorker struct {
	state      *engine.StateManager
	queue      chan<- engine.Action
	peer       tg.InputPeerClass
	interval   time.Duration
	cfgReader  ConfigReader
	telegramID int64
	log        *slog.Logger
}

func NewAutoHuntWorker(state *engine.StateManager, queue chan<- engine.Action, peer tg.InputPeerClass, interval time.Duration, cfgReader ConfigReader, telegramID int64, log *slog.Logger) *AutoHuntWorker {
	return &AutoHuntWorker{state: state, queue: queue, peer: peer, interval: interval, cfgReader: cfgReader, telegramID: telegramID, log: log}
}

func (w *AutoHuntWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.debug("skip hunt: invalid interval", w.interval.String())
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap := w.state.Snapshot()
			if snap.State != parser.StateMainMenu && snap.State != parser.StateDungeon {
				w.debug("skip hunt: state", string(snap.State))
				continue
			}
			if !w.isEnabled(ctx) {
				w.debug("skip hunt: disabled", "")
				continue
			}
			if !parser.HasButton(snap.Buttons, "Caçar") {
				w.debug("skip hunt: no button", "Caçar")
				continue
			}
			select {
			case w.queue <- engine.Action{Type: engine.ActionClick, Label: "Caçar", Peer: w.peer, Reason: "worker_hunt"}:
				w.debug("enqueue", "Caçar")
			default:
				w.debug("queue full, skipping", "Caçar")
			}
		}
	}
}

func (w *AutoHuntWorker) isEnabled(ctx context.Context) bool {
	if w.cfgReader == nil || w.telegramID == 0 {
		return true
	}
	cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID)
	if err != nil {
		return false
	}
	return cfg.AutoHunt
}

func (w *AutoHuntWorker) debug(msg, val string) {
	if w.log == nil {
		return
	}
	if val != "" {
		w.log.Debug(msg, slog.String("worker", "hunt"), slog.String("value", val))
		return
	}
	w.log.Debug(msg, slog.String("worker", "hunt"))
}
