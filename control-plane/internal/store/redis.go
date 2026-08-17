package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// KeyPrefix namespaces every Control Plane key. Production shares the existing
// Redis with other platforms, so AI keys stay under `ai:` (ARCHITECTURE-v1
// section 7).
const KeyPrefix = "ai:"

// OpenRedis dials Redis and blocks until it answers PING or timeout elapses.
func OpenRedis(ctx context.Context, addr, password string, db int, timeout time.Duration, log *slog.Logger) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	probe := func(ctx context.Context) error { return client.Ping(ctx).Err() }
	if err := waitReady(ctx, timeout, log, "redis", probe); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect redis: %w", err)
	}
	log.Info("redis connected", "address", addr, "db", db, "keyPrefix", KeyPrefix)
	return client, nil
}
