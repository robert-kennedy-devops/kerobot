package engine

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gotd/td/tg"

	"kerobot/internal/telegram"
	"kerobot/pkg/metrics"
	"kerobot/pkg/retry"
)

type ActionType string

const (
	ActionClick ActionType = "CLICK"
	ActionSend  ActionType = "SEND"
)

type Action struct {
	Type   ActionType
	Label  string
	Text   string
	Peer   tg.InputPeerClass
	Reason string
}

type Executor struct {
	client  *telegram.Client
	log     *slog.Logger
	queue   chan Action
	delay   time.Duration
	retry   retry.Config
	metrics *metrics.Counters
}

func NewExecutor(client *telegram.Client, log *slog.Logger, delay time.Duration, retryCfg retry.Config, counters *metrics.Counters) *Executor {
	return &Executor{
		client:  client,
		log:     log,
		queue:   make(chan Action, 200),
		delay:   delay,
		retry:   retryCfg,
		metrics: counters,
	}
}

func (e *Executor) Queue() chan<- Action { return e.queue }

func (e *Executor) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case action := <-e.queue:
				e.execute(ctx, action)
				if e.delay > 0 {
					time.Sleep(e.delay)
				}
			}
		}
	}()
}

func (e *Executor) execute(ctx context.Context, action Action) {
	if action.Peer == nil {
		e.log.Warn("action without peer", slog.String("action", string(action.Type)))
		return
	}

	err := retry.Do(ctx, e.retry, func() error {
		switch action.Type {
		case ActionClick:
			if action.Label == "" {
				return errors.New("missing button label")
			}
			return e.client.ClickButton(ctx, action.Peer, action.Label)
		case ActionSend:
			if action.Text == "" {
				return errors.New("missing message text")
			}
			return e.client.SendMessage(ctx, action.Peer, action.Text)
		default:
			return errors.New("unknown action type")
		}
	})

	if err != nil {
		if e.metrics != nil {
			e.metrics.IncActionsError()
		}
		e.log.Error("action failed", slog.String("type", string(action.Type)), slog.String("label", action.Label), slog.String("reason", action.Reason), slog.Any("err", err))
		return
	}

	if e.metrics != nil {
		e.metrics.IncActionsOK()
	}
	e.log.Info("action ok", slog.String("type", string(action.Type)), slog.String("label", action.Label), slog.String("reason", action.Reason))
}
