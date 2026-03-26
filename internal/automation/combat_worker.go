package automation

import (
	"context"
	"log/slog"
	"time"

	"github.com/gotd/td/tg"

	"kerobot/internal/engine"
	"kerobot/internal/parser"
)

type AutoCombatWorker struct {
	state      *engine.StateManager
	queue      chan<- engine.Action
	peer       tg.InputPeerClass
	interval   time.Duration
	cfgReader  ConfigReader
	telegramID int64
	log        *slog.Logger
}

func NewAutoCombatWorker(state *engine.StateManager, queue chan<- engine.Action, peer tg.InputPeerClass, interval time.Duration, cfgReader ConfigReader, telegramID int64, log *slog.Logger) *AutoCombatWorker {
	return &AutoCombatWorker{state: state, queue: queue, peer: peer, interval: interval, cfgReader: cfgReader, telegramID: telegramID, log: log}
}

func (w *AutoCombatWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.debug("skip combat: invalid interval", w.interval.String())
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
			if snap.State != parser.StateCombat {
				w.debug("skip combat: state", string(snap.State))
				continue
			}
			if !w.isEnabled(ctx) {
				w.debug("skip combat: disabled", "")
				continue
			}
			if !parser.HasButton(snap.Buttons, "Atacar") {
				w.debug("skip combat: no button", "Atacar")
				continue
			}
			select {
			case w.queue <- engine.Action{Type: engine.ActionClick, Label: "Atacar", Peer: w.peer, Reason: "worker_combat"}:
				w.debug("enqueue", "Atacar")
			default:
				w.debug("queue full, skipping", "Atacar")
			}
		}
	}
}

func (w *AutoCombatWorker) isEnabled(ctx context.Context) bool {
	if w.cfgReader == nil || w.telegramID == 0 {
		return true
	}
	cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID)
	if err != nil {
		return false
	}
	return cfg.AutoCombat
}

func (w *AutoCombatWorker) debug(msg, val string) {
	if w.log == nil {
		return
	}
	if val != "" {
		w.log.Debug(msg, slog.String("worker", "combat"), slog.String("value", val))
		return
	}
	w.log.Debug(msg, slog.String("worker", "combat"))
}
