package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidKey is the only authentication failure callers ever see. The
// specific reason (unknown prefix, wrong secret, revoked, expired) stays in the
// server log so a probe cannot learn which prefixes exist.
var ErrInvalidKey = errors.New("invalid api key")

// ErrKeyNotFound is returned by administrative lookups, where the caller is
// already authorized to know whether a key exists.
var ErrKeyNotFound = errors.New("api key not found")

// Identity is the authenticated caller attached to a request context.
type Identity struct {
	KeyID              string   `json:"keyId"`
	Name               string   `json:"name"`
	Scopes             []string `json:"scopes"`
	CompanyID          string   `json:"companyId,omitempty"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute"`
}

func (i Identity) HasScope(scope string) bool {
	for _, granted := range i.Scopes {
		if granted == scope {
			return true
		}
	}
	return false
}

// Record is an API key as an administrator sees it: never the secret.
type Record struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Prefix             string     `json:"prefix"`
	Scopes             []string   `json:"scopes"`
	CompanyID          string     `json:"companyId,omitempty"`
	RateLimitPerMinute int        `json:"rateLimitPerMinute"`
	CreatedBy          string     `json:"createdBy"`
	CreatedAt          time.Time  `json:"createdAt"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	LastUsedAt         *time.Time `json:"lastUsedAt,omitempty"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
}

type CreateParams struct {
	Name               string
	Scopes             []string
	CompanyID          string
	RateLimitPerMinute int
	CreatedBy          string
	ExpiresAt          *time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const recordColumns = `id::text, name, prefix, scopes, coalesce(company_id, ''),
	rate_limit_per_minute, created_by, created_at, expires_at, last_used_at, revoked_at`

// Create mints a key and returns the raw value. That value is the only copy:
// it is not stored and must be shown to the operator exactly once.
func (s *Store) Create(ctx context.Context, params CreateParams) (Record, string, error) {
	scopes, err := NormalizeScopes(params.Scopes)
	if err != nil {
		return Record{}, "", err
	}
	if params.Name == "" {
		return Record{}, "", fmt.Errorf("api key name is required")
	}
	if params.RateLimitPerMinute <= 0 {
		return Record{}, "", fmt.Errorf("rate limit must be greater than zero")
	}

	generated, err := Generate()
	if err != nil {
		return Record{}, "", err
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO api_keys (name, prefix, secret_hash, scopes, company_id,
		                      rate_limit_per_minute, created_by, expires_at)
		VALUES ($1, $2, $3, $4, nullif($5, ''), $6, $7, $8)
		RETURNING `+recordColumns,
		params.Name, generated.Prefix, generated.SecretHash, scopes, params.CompanyID,
		params.RateLimitPerMinute, params.CreatedBy, params.ExpiresAt)

	record, err := scanRecord(row)
	if err != nil {
		return Record{}, "", fmt.Errorf("create api key: %w", err)
	}
	return record, generated.Secret, nil
}

// Authenticate verifies a presented key and returns the caller's identity.
//
// The reason for a failure is returned alongside ErrInvalidKey for logging but
// must not be sent to the client.
func (s *Store) Authenticate(ctx context.Context, presented string) (Identity, string, error) {
	prefix, secret, err := Parse(presented)
	if err != nil {
		return Identity{}, "malformed key", ErrInvalidKey
	}

	var (
		id        string
		name      string
		hash      string
		scopes    []string
		companyID string
		limit     int
		expiresAt *time.Time
		revokedAt *time.Time
	)
	err = s.pool.QueryRow(ctx, `
		SELECT id::text, name, secret_hash, scopes, coalesce(company_id, ''),
		       rate_limit_per_minute, expires_at, revoked_at
		FROM api_keys
		WHERE prefix = $1 AND deleted_at IS NULL`, prefix).
		Scan(&id, &name, &hash, &scopes, &companyID, &limit, &expiresAt, &revokedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Identity{}, "unknown prefix", ErrInvalidKey
	case err != nil:
		return Identity{}, "", fmt.Errorf("look up api key: %w", err)
	}

	// Compare the secret even when the key is unusable, so a revoked or expired
	// key does not answer faster than a wrong secret.
	matches := SecretMatches(secret, hash)
	switch {
	case !matches:
		return Identity{}, "secret mismatch", ErrInvalidKey
	case revokedAt != nil:
		return Identity{}, "key revoked", ErrInvalidKey
	case expiresAt != nil && expiresAt.Before(time.Now()):
		return Identity{}, "key expired", ErrInvalidKey
	}

	return Identity{
		KeyID:              id,
		Name:               name,
		Scopes:             scopes,
		CompanyID:          companyID,
		RateLimitPerMinute: limit,
	}, "", nil
}

// TouchLastUsed records recent activity. It is throttled to one write a minute
// per key so a busy caller does not turn every request into a database write.
func (s *Store) TouchLastUsed(ctx context.Context, keyID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys SET last_used_at = now()
		WHERE id = $1::uuid
		  AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, keyID)
	return err
}

func (s *Store) List(ctx context.Context) ([]Record, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+recordColumns+`
		FROM api_keys WHERE deleted_at IS NULL ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("list api keys: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	return records, nil
}

// Revoke is idempotent: revoking an already revoked key keeps the original
// timestamp so the audit trail stays truthful.
func (s *Store) Revoke(ctx context.Context, id string) (Record, error) {
	row := s.pool.QueryRow(ctx, `
		UPDATE api_keys SET revoked_at = coalesce(revoked_at, now())
		WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING `+recordColumns, id)

	record, err := scanRecord(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Record{}, ErrKeyNotFound
	case err != nil:
		return Record{}, fmt.Errorf("revoke api key: %w", err)
	}
	return record, nil
}

// scannable covers both pgx.Row and pgx.Rows.
type scannable interface {
	Scan(dest ...any) error
}

func scanRecord(row scannable) (Record, error) {
	var record Record
	err := row.Scan(&record.ID, &record.Name, &record.Prefix, &record.Scopes, &record.CompanyID,
		&record.RateLimitPerMinute, &record.CreatedBy, &record.CreatedAt,
		&record.ExpiresAt, &record.LastUsedAt, &record.RevokedAt)
	return record, err
}
