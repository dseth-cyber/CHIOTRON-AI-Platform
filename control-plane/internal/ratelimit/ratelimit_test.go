package ratelimit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newLimiter(t *testing.T) (*Limiter, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return New(client), server
}

func TestAllowPermitsUpToTheLimit(t *testing.T) {
	limiter, _ := newLimiter(t)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		decision, err := limiter.Allow(ctx, "key-a", 3)
		if err != nil {
			t.Fatalf("Allow() returned error: %v", err)
		}
		if !decision.Allowed {
			t.Fatalf("request %d was denied, want allowed within the limit", i)
		}
		if want := 3 - i; decision.Remaining != want {
			t.Errorf("request %d remaining = %d, want %d", i, decision.Remaining, want)
		}
	}

	decision, err := limiter.Allow(ctx, "key-a", 3)
	if err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}
	if decision.Allowed {
		t.Error("the fourth request was allowed, want denied")
	}
	if decision.Remaining != 0 {
		t.Errorf("remaining = %d, want 0 once the limit is spent", decision.Remaining)
	}
}

// One key exhausting its quota must not affect another.
func TestAllowIsolatesSubjects(t *testing.T) {
	limiter, _ := newLimiter(t)
	ctx := context.Background()

	if _, err := limiter.Allow(ctx, "key-a", 1); err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}
	if decision, _ := limiter.Allow(ctx, "key-a", 1); decision.Allowed {
		t.Error("key-a was allowed past its limit")
	}

	decision, err := limiter.Allow(ctx, "key-b", 1)
	if err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}
	if !decision.Allowed {
		t.Error("key-b was denied because key-a was throttled")
	}
}

// Production shares Redis with other platforms, so counters stay under `ai:`.
func TestAllowNamespacesKeys(t *testing.T) {
	limiter, server := newLimiter(t)

	if _, err := limiter.Allow(context.Background(), "key-a", 1); err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}
	keys := server.Keys()
	if len(keys) != 1 {
		t.Fatalf("stored %d keys, want 1", len(keys))
	}
	if !strings.HasPrefix(keys[0], "ai:rate-limit:") {
		t.Errorf("key %q is not under the ai:rate-limit: namespace", keys[0])
	}
}

// Without a TTL a counter would outlive its window and permanently throttle a key.
func TestAllowSetsExpiry(t *testing.T) {
	limiter, server := newLimiter(t)

	if _, err := limiter.Allow(context.Background(), "key-a", 1); err != nil {
		t.Fatalf("Allow() returned error: %v", err)
	}
	ttl := server.TTL(server.Keys()[0])
	if ttl <= 0 {
		t.Fatalf("counter TTL = %v, want a positive expiry", ttl)
	}
	if ttl > 2*time.Minute {
		t.Errorf("counter TTL = %v, want at most two windows", ttl)
	}
}

func TestRetryAfterIsAtLeastOneSecond(t *testing.T) {
	now := time.Now()
	decision := Decision{ResetAt: now.Add(-time.Second)}

	if got := decision.RetryAfter(now); got != 1 {
		t.Errorf("RetryAfter() = %d for an elapsed window, want 1", got)
	}
	if got := (Decision{ResetAt: now.Add(30 * time.Second)}).RetryAfter(now); got != 30 {
		t.Errorf("RetryAfter() = %d, want 30", got)
	}
}
