// Package store owns the Control Plane's connections to its backing services.
// The AI Platform owns this database; it is separate from ERP service
// databases even when the same cluster operates it (ARCHITECTURE-v1 section 8).
package store

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// OpenPostgres dials the AI database and blocks until it answers or timeout
// elapses. Compose starts the API once PostgreSQL reports healthy, but a
// production Control Plane may start before its database is reachable, so the
// retry loop is part of normal startup rather than an error path.
func OpenPostgres(ctx context.Context, url string, timeout time.Duration, log *slog.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse AI_DATABASE_URL: %w", err)
	}
	cfg.MaxConnLifetime = 30 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := waitReady(ctx, timeout, log, "postgres", pool.Ping); err != nil {
		pool.Close()
		return nil, err
	}
	log.Info("postgres connected", "database", cfg.ConnConfig.Database, "host", cfg.ConnConfig.Host)
	return pool, nil
}

// waitReady retries probe until it succeeds, the deadline passes or ctx ends.
func waitReady(ctx context.Context, timeout time.Duration, log *slog.Logger, name string, probe func(context.Context) error) error {
	deadline := time.Now().Add(timeout)
	attempt := 0
	for {
		attempt++
		attemptCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := probe(attemptCtx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s not reachable after %s: %w", name, timeout, err)
		}
		log.Warn("dependency not ready, retrying", "dependency", name, "attempt", attempt, "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
