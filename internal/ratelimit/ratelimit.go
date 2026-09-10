// Package ratelimit implements a shared, Valkey-backed rate limiter for
// BookDB API keys. It uses a fixed-window counter per key, stored in Valkey
// so that multiple API replicas share the same limit.
//
// When Valkey is unavailable, the limiter returns ErrUnavailable so the
// caller can respond with a documented 503.
package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrUnavailable is returned when the Valkey backend is not reachable.
var ErrUnavailable = errors.New("ratelimit: backend unavailable")

// Limiter is a shared, Valkey-backed rate limiter.
type Limiter struct {
	rdb    *redis.Client
	prefix string
	limit  int
	window time.Duration
}

// New creates a Limiter backed by the given Valkey/Redis client.
// limit is the maximum number of requests allowed per window per key.
func New(rdb *redis.Client, prefix string, limit int, window time.Duration) *Limiter {
	if prefix == "" {
		prefix = "bookdb:ratelimit"
	}
	return &Limiter{
		rdb:    rdb,
		prefix: prefix,
		limit:  limit,
		window: window,
	}
}

// Allow checks whether the given key is within its rate limit. It returns
// true if the request is allowed, false if the limit is exceeded. It returns
// ErrUnavailable when Valkey is not reachable.
func (l *Limiter) Allow(ctx context.Context, key string) (bool, error) {
	if l.rdb == nil {
		return false, ErrUnavailable
	}

	// Use a fixed-window counter. The window key includes the current
	// window start so counters reset automatically.
	now := time.Now().UTC()
	windowStart := now.Truncate(l.window)
	redisKey := fmt.Sprintf("%s:%s:%d", l.prefix, key, windowStart.Unix())

	// INCR is atomic and returns the new count. Set an expiry on the key
	// so it is cleaned up after the window passes.
	pipe := l.rdb.TxPipeline()
	incr := pipe.Incr(ctx, redisKey)
	pipe.Expire(ctx, redisKey, l.window+time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, fmt.Errorf("ratelimit: pipeline: %w", err)
	}

	count, err := incr.Result()
	if err != nil {
		return false, fmt.Errorf("ratelimit: incr: %w", err)
	}

	return count <= int64(l.limit), nil
}

// RetryAfter returns the duration until the current window resets, for use
// in the Retry-After header when the limit is exceeded.
func (l *Limiter) RetryAfter() time.Duration {
	now := time.Now().UTC()
	windowStart := now.Truncate(l.window)
	return windowStart.Add(l.window).Sub(now)
}

// Limit returns the configured per-window limit.
func (l *Limiter) Limit() int {
	return l.limit
}
