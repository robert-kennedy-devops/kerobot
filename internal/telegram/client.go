package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/updates"
	"github.com/gotd/td/tg"
	"golang.org/x/time/rate"

	"kerobot/pkg/textutil"
)

type Client struct {
	apiID       int
	apiHash     string
	phone       string
	password    string
	code        string
	sessionPath string

	client  *telegram.Client
	api     *tg.Client
	limiter *rate.Limiter

	mu      sync.RWMutex
	lastMsg map[int64]*Message
	ready   chan struct{}
}

type Message struct {
	ChatID  int64
	MsgID   int
	Text    string
	Buttons []InlineButton
	Raw     tg.MessageClass
}

type InlineButton struct {
	Text string
	Data []byte
}

func NewClient(apiID int, apiHash, phone, password, code, sessionPath string, ratePerSecond int) *Client {
	if ratePerSecond <= 0 {
		ratePerSecond = 1
	}
	return &Client{
		apiID:       apiID,
		apiHash:     apiHash,
		phone:       phone,
		password:    password,
		code:        code,
		sessionPath: sessionPath,
		limiter:     rate.NewLimiter(rate.Limit(ratePerSecond), 1),
		lastMsg:     make(map[int64]*Message),
		ready:       make(chan struct{}),
	}
}

func (c *Client) Start(ctx context.Context, onMessage func(context.Context, *Message) error) error {
	slog.Debug("telegram: start")
	dispatcher := tg.NewUpdateDispatcher()
	updatesHandler := updates.New(updates.Config{Handler: dispatcher})

	c.client = telegram.NewClient(c.apiID, c.apiHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: c.sessionPath},
		UpdateHandler:  updatesHandler,
	})

	dispatcher.OnNewMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateNewMessage) error {
		return c.handleMessage(ctx, update.Message, onMessage)
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateNewChannelMessage) error {
		return c.handleMessage(ctx, update.Message, onMessage)
	})
	dispatcher.OnEditMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateEditMessage) error {
		return c.handleMessage(ctx, update.Message, onMessage)
	})
	dispatcher.OnEditChannelMessage(func(ctx context.Context, entities tg.Entities, update *tg.UpdateEditChannelMessage) error {
		return c.handleMessage(ctx, update.Message, onMessage)
	})

	return c.client.Run(ctx, func(ctx context.Context) error {
		slog.Debug("telegram: auth begin")
		if err := c.authIfNeeded(ctx); err != nil {
			return err
		}
		slog.Debug("telegram: auth ok")
		c.api = c.client.API()
		slog.Debug("telegram: api ready")
		close(c.ready)
		<-ctx.Done()
		return ctx.Err()
	})
}

func (c *Client) Ready() <-chan struct{} { return c.ready }

func (c *Client) authIfNeeded(ctx context.Context) error {
	slog.Debug("telegram: authIfNeeded")
	if c.phone == "" {
		if _, err := os.Stat(c.sessionPath); err != nil {
			return errors.New("missing login session; use QR login to create one")
		}
	}
	code := auth.CodeAuthenticatorFunc(func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		if c.code == "" {
			return ReadCodeFromStdin()
		}
		return c.code, nil
	})
	flow := auth.NewFlow(auth.Constant(c.phone, c.password, code), auth.SendCodeOptions{})
	return c.client.Auth().IfNecessary(ctx, flow)
}

func (c *Client) SendMessage(ctx context.Context, peer tg.InputPeerClass, text string) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	if c.api == nil {
		return errors.New("telegram api not ready")
	}
	_, err := c.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:     peer,
		Message:  text,
		RandomID: time.Now().UnixNano(),
	})
	return err
}

func (c *Client) ClickButton(ctx context.Context, peer tg.InputPeerClass, label string) error {
	if err := c.limiter.Wait(ctx); err != nil {
		return err
	}
	if c.api == nil {
		return errors.New("telegram api not ready")
	}

	c.mu.RLock()
	msg := c.lastMsg[peerID(peer)]
	c.mu.RUnlock()
	if msg == nil {
		return errors.New("no cached message for peer")
	}

	for _, b := range msg.Buttons {
		if b.Text == label || matchButton(b.Text, label) {
			_, err := c.api.MessagesGetBotCallbackAnswer(ctx, &tg.MessagesGetBotCallbackAnswerRequest{
				Peer:  peer,
				MsgID: msg.MsgID,
				Data:  b.Data,
			})
			if err != nil {
				return fmt.Errorf("callback %s: %w", label, err)
			}
			return nil
		}
	}
	return fmt.Errorf("button not found: %s", label)
}

func (c *Client) GetLastMessage(chatID int64) *Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastMsg[chatID]
}

func (c *Client) GetButtons(chatID int64) []InlineButton {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if msg := c.lastMsg[chatID]; msg != nil {
		return msg.Buttons
	}
	return nil
}

func (c *Client) ResolvePeerByUsername(ctx context.Context, username string) (tg.InputPeerClass, error) {
	if c.api == nil {
		return nil, errors.New("telegram api not ready")
	}
	res, err := c.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{Username: username})
	if err != nil {
		return nil, err
	}
	if res.Peer == nil {
		return nil, fmt.Errorf("peer not found: %s", username)
	}
	switch p := res.Peer.(type) {
	case *tg.PeerUser:
		for _, u := range res.Users {
			if user, ok := u.(*tg.User); ok && user.ID == p.UserID {
				return &tg.InputPeerUser{UserID: p.UserID, AccessHash: user.AccessHash}, nil
			}
		}
	case *tg.PeerChannel:
		for _, ch := range res.Chats {
			if channel, ok := ch.(*tg.Channel); ok && channel.ID == p.ChannelID {
				return &tg.InputPeerChannel{ChannelID: p.ChannelID, AccessHash: channel.AccessHash}, nil
			}
		}
	case *tg.PeerChat:
		return &tg.InputPeerChat{ChatID: p.ChatID}, nil
	}
	return nil, fmt.Errorf("unable to resolve access hash for %s", username)
}

func (c *Client) handleMessage(ctx context.Context, msg tg.MessageClass, onMessage func(context.Context, *Message) error) error {
	m, ok := msg.(*tg.Message)
	if !ok {
		return nil
	}
	buttons := extractButtons(m)
	chatID := messagePeerID(m)
	ev := &Message{
		ChatID:  chatID,
		MsgID:   m.ID,
		Text:    m.Message,
		Buttons: buttons,
		Raw:     msg,
	}

	c.mu.Lock()
	c.lastMsg[chatID] = ev
	c.mu.Unlock()

	if onMessage != nil {
		return onMessage(ctx, ev)
	}
	return nil
}

func messagePeerID(m *tg.Message) int64 {
	if m.PeerID == nil {
		return 0
	}
	switch peer := m.PeerID.(type) {
	case *tg.PeerUser:
		return int64(peer.UserID)
	case *tg.PeerChat:
		return int64(peer.ChatID)
	case *tg.PeerChannel:
		return int64(peer.ChannelID)
	default:
		return 0
	}
}

func peerID(peer tg.InputPeerClass) int64 {
	switch p := peer.(type) {
	case *tg.InputPeerUser:
		return int64(p.UserID)
	case *tg.InputPeerChat:
		return int64(p.ChatID)
	case *tg.InputPeerChannel:
		return int64(p.ChannelID)
	default:
		return 0
	}
}

func PeerID(peer tg.InputPeerClass) int64 {
	return peerID(peer)
}

func matchButton(actual, desired string) bool {
	a := textutil.Normalize(actual)
	d := textutil.Normalize(desired)
	return a == d || strings.Contains(a, d) || strings.Contains(d, a)
}
