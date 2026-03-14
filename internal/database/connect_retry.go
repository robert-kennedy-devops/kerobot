package database

import (
	"context"
	"fmt"
	"time"
)

func ConnectWithRetry(ctx context.Context, dsn string, attempts int, backoff time.Duration) (*DB, error) {
	if attempts <= 0 {
		attempts = 1
	}
	if backoff <= 0 {
		backoff = 1 * time.Second
	}
	var err error
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var db *DB
		db, err = Connect(ctx, dsn)
		if err == nil {
			return db, nil
		}
		if i < attempts-1 {
			sleep := backoff * time.Duration(1<<i)
			if sleep > 30*time.Second {
				sleep = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(sleep):
			}
		}
	}
	return nil, fmt.Errorf("connect with retry: %w", err)
}
