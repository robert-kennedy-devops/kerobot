package telegram

import (
	"context"
	"sync"
)

type Listener struct {
	mu           sync.RWMutex
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
	l.mu.Lock()
	l.targetChatID = chatID
	l.mu.Unlock()
}

func (l *Listener) Handle(ctx context.Context, msg *Message) error {
	if msg == nil {
		return nil
	}
	l.mu.RLock()
	target := l.targetChatID
	l.mu.RUnlock()
	// If no target set yet, drop messages to avoid mixing chats.
	if target == 0 {
		return nil
	}
	if msg.ChatID != target {
		return nil
	}
	select {
	case l.events <- msg:
	case <-ctx.Done():
	}
	return nil
}
