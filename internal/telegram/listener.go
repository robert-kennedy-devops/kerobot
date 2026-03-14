package telegram

import (
	"context"
)

type Listener struct {
	targetChatID int64
	events       chan *Message
}

func NewListener(targetChatID int64) *Listener {
	return &Listener{
		targetChatID: targetChatID,
		events:       make(chan *Message, 200),
	}
}

func (l *Listener) Events() <-chan *Message { return l.events }

func (l *Listener) SetTarget(chatID int64) {
	l.targetChatID = chatID
}

func (l *Listener) Handle(ctx context.Context, msg *Message) error {
	if msg == nil {
		return nil
	}
	if l.targetChatID != 0 && msg.ChatID != l.targetChatID {
		return nil
	}
	select {
	case l.events <- msg:
	case <-ctx.Done():
	}
	return nil
}
