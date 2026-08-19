// Package settings owns platform configuration values stored in PostgreSQL.
package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("setting not found")

type Setting struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"` // Stored as raw JSON text
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   string    `json:"updatedBy"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// List returns all platform settings.
func (s *Store) List(ctx context.Context) ([]Setting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT key, value::text, description, updated_at, updated_by
		FROM platform_settings
		ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer rows.Close()

	var result []Setting
	for rows.Next() {
		var st Setting
		if err := rows.Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt, &st.UpdatedBy); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		result = append(result, st)
	}
	return result, rows.Err()
}

// Get returns a single platform setting by key.
func (s *Store) Get(ctx context.Context, key string) (Setting, error) {
	var st Setting
	err := s.pool.QueryRow(ctx, `
		SELECT key, value::text, description, updated_at, updated_by
		FROM platform_settings
		WHERE key = $1`, key).Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt, &st.UpdatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Setting{}, ErrNotFound
		}
		return Setting{}, fmt.Errorf("get setting %q: %w", key, err)
	}
	return st, nil
}

// Set inserts or updates a platform setting.
func (s *Store) Set(ctx context.Context, key string, jsonValue string, description string, updatedBy string) (Setting, error) {
	var st Setting
	err := s.pool.QueryRow(ctx, `
		INSERT INTO platform_settings (key, value, description, updated_at, updated_by)
		VALUES ($1, $2::jsonb, $3, now(), $4)
		ON CONFLICT (key) DO UPDATE
			SET value = EXCLUDED.value,
			    description = CASE WHEN EXCLUDED.description <> '' THEN EXCLUDED.description ELSE platform_settings.description END,
			    updated_at = now(),
			    updated_by = EXCLUDED.updated_by
		RETURNING key, value::text, description, updated_at, updated_by`,
		key, jsonValue, description, updatedBy).Scan(&st.Key, &st.Value, &st.Description, &st.UpdatedAt, &st.UpdatedBy)
	if err != nil {
		return Setting{}, fmt.Errorf("set setting %q: %w", key, err)
	}
	return st, nil
}
