package automation

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/gotd/td/tg"

	"kerobot/internal/engine"
	"kerobot/internal/parser"
)

type AutoHealWorker struct {
	state      *engine.StateManager
	queue      chan<- engine.Action
	peer       tg.InputPeerClass
	interval   time.Duration
	threshold  int
	cfgReader  ConfigReader
	telegramID int64
	log        *slog.Logger
}

func NewAutoHealWorker(state *engine.StateManager, queue chan<- engine.Action, peer tg.InputPeerClass, interval time.Duration, threshold int, cfgReader ConfigReader, telegramID int64, log *slog.Logger) *AutoHealWorker {
	return &AutoHealWorker{state: state, queue: queue, peer: peer, interval: interval, threshold: threshold, cfgReader: cfgReader, telegramID: telegramID, log: log}
}

func (w *AutoHealWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.debug("skip heal: invalid interval", w.interval.String())
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
			if !w.isEnabled(ctx) {
				w.debug("skip heal: disabled", "")
				continue
			}
			if len(snap.Buttons) == 0 {
				w.debug("skip heal: no buttons", "")
				continue
			}
			threshold := w.threshold
			if w.cfgReader != nil && w.telegramID != 0 {
				if cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID); err == nil {
					if cfg.HealPercent > 0 {
						threshold = cfg.HealPercent
					}
				}
			}
			if snap.HPPercent > 0 && snap.HPPercent < threshold {
				var action engine.Action
				switch {
				case parser.HasButton(snap.Buttons, "Consumíveis"):
					action = engine.Action{Type: engine.ActionClick, Label: "Consumíveis", Peer: w.peer, Reason: "open_consumables"}
				case parser.HasButton(snap.Buttons, "Poção de Vida"):
					action = engine.Action{Type: engine.ActionClick, Label: "Poção de Vida", Peer: w.peer, Reason: "use_potion"}
				case parser.HasButton(snap.Buttons, "Inventário"):
					action = engine.Action{Type: engine.ActionClick, Label: "Inventário", Peer: w.peer, Reason: "open_inventory"}
				default:
					w.debug("heal: no buttons", "")
					continue
				}
				select {
				case w.queue <- action:
					w.debug("enqueue", action.Label)
				default:
					w.debug("queue full, skipping", action.Label)
				}
			} else {
				w.debug("skip heal: hp", strconv.Itoa(snap.HPPercent))
			}
		}
	}
}

func (w *AutoHealWorker) isEnabled(ctx context.Context) bool {
	if w.cfgReader == nil || w.telegramID == 0 {
		return true
	}
	cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID)
	if err != nil {
		return false
	}
	return cfg.AutoHeal
}

func (w *AutoHealWorker) debug(msg, val string) {
	if w.log == nil {
		return
	}
	if val != "" {
		w.log.Debug(msg, slog.String("worker", "heal"), slog.String("value", val))
		return
	}
	w.log.Debug(msg, slog.String("worker", "heal"))
}
