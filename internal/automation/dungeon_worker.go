package automation

import (
	"context"
	"log/slog"
	"time"

	"github.com/gotd/td/tg"

	"kerobot/internal/engine"
	"kerobot/internal/parser"
)

type DungeonWorker struct {
	state      *engine.StateManager
	queue      chan<- engine.Action
	peer       tg.InputPeerClass
	interval   time.Duration
	cfgReader  ConfigReader
	telegramID int64
	log        *slog.Logger
}

func NewDungeonWorker(state *engine.StateManager, queue chan<- engine.Action, peer tg.InputPeerClass, interval time.Duration, cfgReader ConfigReader, telegramID int64, log *slog.Logger) *DungeonWorker {
	return &DungeonWorker{state: state, queue: queue, peer: peer, interval: interval, cfgReader: cfgReader, telegramID: telegramID, log: log}
}

func (w *DungeonWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !w.isEnabled(ctx) {
				w.debug("skip dungeon: disabled", "")
				continue
			}
			snap := w.state.Snapshot()
			if snap.State == parser.StateMainMenu || snap.State == parser.StateDungeon {
				actions := []engine.Action{
					{Type: engine.ActionClick, Label: "Masmorra", Peer: w.peer, Reason: "open_dungeon"},
					{Type: engine.ActionClick, Label: "Criar sala", Peer: w.peer, Reason: "create_room"},
					{Type: engine.ActionClick, Label: "Iniciar", Peer: w.peer, Reason: "start_dungeon"},
				}
				dropped := false
				for _, a := range actions {
					select {
					case w.queue <- a:
					default:
						dropped = true
					}
				}
				if dropped {
					w.debug("queue full, some dungeon actions dropped", "")
				} else {
					w.debug("enqueue", "Masmorra")
				}
			} else {
				w.debug("skip dungeon: state", string(snap.State))
			}
		}
	}
}

func (w *DungeonWorker) isEnabled(ctx context.Context) bool {
	if w.cfgReader == nil || w.telegramID == 0 {
		return true
	}
	cfg, err := w.cfgReader.GetConfig(ctx, w.telegramID)
	if err != nil {
		return false
	}
	return cfg.AutoDungeon
}

func (w *DungeonWorker) debug(msg, val string) {
	if w.log == nil {
		return
	}
	if val != "" {
		w.log.Debug(msg, slog.String("worker", "dungeon"), slog.String("value", val))
		return
	}
	w.log.Debug(msg, slog.String("worker", "dungeon"))
}
