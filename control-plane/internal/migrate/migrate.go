// Package migrate applies service-owned SQL migrations at startup.
//
// The AI Platform owns its database schema (ARCHITECTURE-v1 section 8), so
// schema changes ship with the service rather than with the PostgreSQL image.
// The container entrypoint script in infra/postgres only runs on an empty data
// volume and cannot express change over time; this package can.
package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// advisoryLockKey serialises migration runs across Control Plane replicas.
// The value is arbitrary but must stay stable for the lock to be meaningful.
const advisoryLockKey int64 = 8264113001

const createVersionTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  checksum TEXT NOT NULL,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

type migration struct {
	version  int64
	name     string
	checksum string
	body     string
}

// Run applies every migration in dir that has not been recorded yet and
// returns the number applied. It is safe to call on every start: already
// applied migrations are skipped, and a migration whose file changed after
// being applied is reported as an error instead of silently ignored.
func Run(ctx context.Context, pool *pgxpool.Pool, files fs.FS, dir string, log *slog.Logger) (int, error) {
	pending, err := load(files, dir)
	if err != nil {
		return 0, err
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return 0, fmt.Errorf("acquire migration connection: %w", err)
	}
	defer conn.Release()

	// Hold the lock on a single connection for the whole run so a concurrent
	// replica waits rather than racing on the same statements.
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		return 0, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", advisoryLockKey); err != nil {
			log.Error("release migration lock", "error", err)
		}
	}()

	if _, err := conn.Exec(ctx, createVersionTable); err != nil {
		return 0, fmt.Errorf("create schema_migrations: %w", err)
	}

	applied, err := appliedChecksums(ctx, conn)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, m := range pending {
		if recorded, ok := applied[m.version]; ok {
			if recorded != m.checksum {
				return count, fmt.Errorf("migration %04d_%s was already applied with a different checksum; "+
					"edit history is not allowed, add a new migration instead", m.version, m.name)
			}
			continue
		}
		if err := apply(ctx, conn, m); err != nil {
			return count, err
		}
		log.Info("migration applied", "version", m.version, "name", m.name)
		count++
	}
	return count, nil
}

func apply(ctx context.Context, conn *pgxpool.Conn, m migration) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration %04d: %w", m.version, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, m.body); err != nil {
		return fmt.Errorf("apply migration %04d_%s: %w", m.version, m.name, err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO schema_migrations (version, name, checksum) VALUES ($1, $2, $3)",
		m.version, m.name, m.checksum,
	); err != nil {
		return fmt.Errorf("record migration %04d_%s: %w", m.version, m.name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration %04d_%s: %w", m.version, m.name, err)
	}
	return nil
}

func appliedChecksums(ctx context.Context, conn *pgxpool.Conn) (map[int64]string, error) {
	rows, err := conn.Query(ctx, "SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int64]string)
	for rows.Next() {
		var version int64
		var checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return nil, fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[version] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	return applied, nil
}

// load reads `NNNN_name.sql` files from dir and returns them ordered by version.
func load(files fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(files, dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory %q: %w", dir, err)
	}

	var loaded []migration
	seen := make(map[int64]string)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, name, err := parseName(entry.Name())
		if err != nil {
			return nil, err
		}
		if other, duplicate := seen[version]; duplicate {
			return nil, fmt.Errorf("duplicate migration version %04d in %q and %q", version, other, entry.Name())
		}
		seen[version] = entry.Name()

		body, err := fs.ReadFile(files, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}
		sum := sha256.Sum256(body)
		loaded = append(loaded, migration{
			version:  version,
			name:     name,
			checksum: hex.EncodeToString(sum[:]),
			body:     string(body),
		})
	}
	if len(loaded) == 0 {
		return nil, fmt.Errorf("no migrations found in %q", dir)
	}

	sort.Slice(loaded, func(i, j int) bool { return loaded[i].version < loaded[j].version })
	return loaded, nil
}

func parseName(filename string) (int64, string, error) {
	base := strings.TrimSuffix(filename, ".sql")
	prefix, name, ok := strings.Cut(base, "_")
	if !ok || name == "" {
		return 0, "", fmt.Errorf("migration %q must be named NNNN_description.sql", filename)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("migration %q must start with a numeric version", filename)
	}
	return version, name, nil
}
