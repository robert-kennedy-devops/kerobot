package automation

import (
	"context"
	"log/slog"
	"time"

	"github.com/gotd/td/tg"

	"kerobot/internal/engine"
	"kerobot/internal/parser"
)

type AutoPotionWorker struct {
	state      *engine.StateManager
	queue      chan<- engine.Action
	peer       tg.InputPeerClass
	interval   time.Duration
	minPotions int
	cfgReader  ConfigReader
	telegramID int64
	log        *slog.Logger
}

func NewAutoPotionWorker(state *engine.StateManager, queue chan<- engine.Action, peer tg.InputPeerClass, interval time.Duration, minPotions int, cfgReader ConfigReader, telegramID int64, log *slog.Logger) *AutoPotionWorker {
	return &AutoPotionWorker{state: state, queue: queue, peer: peer, interval: interval, minPotions: minPotions, cfgReader: cfgReader, telegramID: telegramID, log: log}
}

func (w *AutoPotionWorker) Run(ctx context.Context) {
	if w.interval <= 0 {
		w.debug("skip potion: invalid interval", w.interval.String())
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !w.isEnabled(ctx) {
				w.debug("skip potion: disabled", "")
				continue
			}
			minPotions := w.minPotions
			if w.cfgReader != nil && w.telegramID != 0 {
				if cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID); err == nil {
					if cfg.MinPotions > 0 {
						minPotions = cfg.MinPotions
					}
				}
			}
			snap := w.state.Snapshot()
			if snap.Potions < 0 {
				w.debug("skip potion: unknown count", "")
				continue
			}
			if snap.Potions >= minPotions {
				w.debug("skip potion: enough", "")
				continue
			}

			// Smart flow based on visible buttons.
			var action engine.Action
			switch {
			case parser.HasButton(snap.Buttons, "Loja"):
				action = engine.Action{Type: engine.ActionClick, Label: "Loja", Peer: w.peer, Reason: "open_shop"}
			case parser.HasButton(snap.Buttons, "Comprar"):
				action = engine.Action{Type: engine.ActionClick, Label: "Comprar", Peer: w.peer, Reason: "shop_buy"}
			case parser.HasButton(snap.Buttons, "Poção de Vida"):
				action = engine.Action{Type: engine.ActionClick, Label: "Poção de Vida", Peer: w.peer, Reason: "select_potion"}
			case parser.HasButton(snap.Buttons, "Escolher quantidade"):
				action = engine.Action{Type: engine.ActionClick, Label: "Escolher quantidade", Peer: w.peer, Reason: "choose_amount"}
			case parser.HasButton(snap.Buttons, "Comprar 5"):
				action = engine.Action{Type: engine.ActionClick, Label: "Comprar 5", Peer: w.peer, Reason: "buy_5"}
			default:
				continue
			}
			select {
			case w.queue <- action:
				w.debug("enqueue", action.Label)
			default:
				w.debug("queue full, skipping", action.Label)
			}
		}
	}
}

func (w *AutoPotionWorker) isEnabled(ctx context.Context) bool {
	if w.cfgReader == nil || w.telegramID == 0 {
		return true
	}
	cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID)
	if err != nil {
		return false
	}
	return cfg.AutoBuyPotions
}

func (w *AutoPotionWorker) debug(msg, val string) {
	if w.log == nil {
		return
	}
	if val != "" {
		w.log.Debug(msg, slog.String("worker", "potion"), slog.String("value", val))
		return
	}
	w.log.Debug(msg, slog.String("worker", "potion"))
}
