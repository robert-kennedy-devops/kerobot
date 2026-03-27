package engine

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
	Type     ActionType
	Label    string
	Text     string
	Peer     tg.InputPeerClass
	Reason   string
	Priority int // 0=normal, 1=high
}

type Executor struct {
	client  *telegram.Client
	log     *slog.Logger
	queue   chan Action
	highQ   chan Action
	delay   time.Duration
	retry   retry.Config
	metrics *metrics.Counters
	lastRefresh time.Time
	refreshCooldown time.Duration
}

func NewExecutor(client *telegram.Client, log *slog.Logger, delay time.Duration, retryCfg retry.Config, counters *metrics.Counters) *Executor {
	return &Executor{
		client:  client,
		log:     log,
		queue:   make(chan Action, 200),
		highQ:   make(chan Action, 50),
		delay:   delay,
		retry:   retryCfg,
		metrics: counters,
		refreshCooldown: 15 * time.Second,
	}
}

func (e *Executor) Queue() chan<- Action { return e.queue }

func (e *Executor) Enqueue(action Action) {
	if action.Priority > 0 {
		select {
		case e.highQ <- action:
			return
		default:
		}
	}
	select {
	case e.queue <- action:
	default:
		e.log.Warn("action queue full, dropping",
			slog.String("type", string(action.Type)),
			slog.String("label", action.Label),
			slog.String("reason", action.Reason),
		)
	}
}

func (e *Executor) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				select {
				case <-ctx.Done():
					return
				case action := <-e.highQ:
					e.execute(ctx, action)
					if e.delay > 0 {
						time.Sleep(e.delay)
					}
				case action := <-e.queue:
					e.execute(ctx, action)
					if e.delay > 0 {
						time.Sleep(e.delay)
					}
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
		e.maybeRefreshOnDataInvalid(ctx, action, err)
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

func (e *Executor) maybeRefreshOnDataInvalid(ctx context.Context, action Action, err error) {
	if err == nil || action.Peer == nil {
		return
	}
	if action.Type != ActionClick {
		return
	}
	if !strings.Contains(err.Error(), "DATA_INVALID") {
		return
	}
	now := time.Now()
	if now.Sub(e.lastRefresh) < e.refreshCooldown {
		return
	}
	e.lastRefresh = now
	e.log.Warn("data invalid: refreshing with /start", slog.String("reason", action.Reason))
	_ = e.client.SendMessage(ctx, action.Peer, "/start")
}
