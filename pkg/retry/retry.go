package retry

import (
	"context"
	"time"
)

type Config struct {
	Attempts int
	Delay    time.Duration
}

func Do(ctx context.Context, cfg Config, fn func() error) error {
	attempts := cfg.Attempts
	if attempts <= 0 {
		attempts = 1
	}
	var err error
	for i := 0; i < attempts; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err = fn()
		if err == nil {
			return nil
		}
		if i < attempts-1 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.Delay):
			}
		}
	}
	return err
}
