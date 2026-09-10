package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewValkeyClient creates a go-redis client from a Valkey/Redis URL.
// The URL format is redis://host:port/db or redis://user:pass@host:port/db.
func NewValkeyClient(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("ratelimit: parse valkey url: %w", err)
	}
	opts.PoolSize = 10
	opts.MinIdleConns = 2
	opts.DialTimeout = 3 * time.Second
	opts.ReadTimeout = 3 * time.Second
	opts.WriteTimeout = 3 * time.Second

	client := redis.NewClient(opts)
	// Verify connectivity.
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ratelimit: valkey ping: %w", err)
	}
	return client, nil
}
