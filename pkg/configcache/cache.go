package configcache

import (
	"context"
	"sync"
	"time"

	"kerobot/internal/models"
)

// ConfigReader is the subset of database.ConfigRepo used by automation workers.
type ConfigReader interface {
	GetConfig(ctx context.Context, telegramID int64) (models.Config, error)
}

type entry struct {
	cfg models.Config
	exp time.Time
}

// CachedReader wraps a ConfigReader and caches results per telegramID for ttl.
// This avoids a DB round-trip on every worker tick or incoming message.
type CachedReader struct {
	inner ConfigReader
	ttl   time.Duration
	mu    sync.Mutex
	cache map[int64]entry
}

func New(inner ConfigReader, ttl time.Duration) *CachedReader {
	return &CachedReader{inner: inner, ttl: ttl, cache: make(map[int64]entry)}
}

func (c *CachedReader) GetConfig(ctx context.Context, telegramID int64) (models.Config, error) {
	c.mu.Lock()
	if e, ok := c.cache[telegramID]; ok && time.Now().Before(e.exp) {
		cfg := e.cfg
		c.mu.Unlock()
		return cfg, nil
	}
	c.mu.Unlock()

	cfg, err := c.inner.GetConfig(ctx, telegramID)
	if err != nil {
		return cfg, err
	}

	c.mu.Lock()
	c.cache[telegramID] = entry{cfg: cfg, exp: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return cfg, nil
}
