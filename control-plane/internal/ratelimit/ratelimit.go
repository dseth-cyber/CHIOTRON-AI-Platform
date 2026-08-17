// Package ratelimit enforces per-key request quotas in Redis.
//
// Keys live under the `ai:rate-limit:` namespace because production shares the
// existing Redis with other platforms (ARCHITECTURE-v1 section 7). Limits are
// configuration carried on each API key, not a hard-coded business constant.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const namespace = "ai:rate-limit:"

// Decision is the outcome of one limiter check, shaped for the response
// headers a caller needs to back off correctly.
type Decision struct {
	Allowed   bool
	Limit     int
	Remaining int
	ResetAt   time.Time
}

// RetryAfter is how long the caller should wait, rounded up to a whole second.
func (d Decision) RetryAfter(now time.Time) int {
	seconds := int(d.ResetAt.Sub(now).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}

type Limiter struct {
	redis *redis.Client
}

func New(client *redis.Client) *Limiter { return &Limiter{redis: client} }

// Allow applies a fixed one-minute window.
//
// A fixed window can admit up to twice the limit across a window boundary. That
// is accepted here: the counter is a single atomic INCR with no stored history,
// which keeps the hot path to one round trip. A sliding window belongs with the
// quota work in the Gateway phase, where burst behaviour is a policy decision.
func (l *Limiter) Allow(ctx context.Context, subject string, limit int) (Decision, error) {
	now := time.Now()
	window := now.Truncate(time.Minute)
	resetAt := window.Add(time.Minute)
	key := fmt.Sprintf("%s%s:%d", namespace, subject, window.Unix())

	pipeline := l.redis.TxPipeline()
	count := pipeline.Incr(ctx, key)
	// Expiry is set on every call rather than only on creation: an INCR that
	// raced with an eviction would otherwise leave a counter with no TTL.
	pipeline.Expire(ctx, key, 2*time.Minute)
	if _, err := pipeline.Exec(ctx); err != nil {
		return Decision{}, fmt.Errorf("rate limit check: %w", err)
	}

	used := int(count.Val())
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return Decision{
		Allowed:   used <= limit,
		Limit:     limit,
		Remaining: remaining,
		ResetAt:   resetAt,
	}, nil
}
