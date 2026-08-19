// Package compute holds the platform's provider registry: which model backends
// exist, how to reach them, what they may be told, and which logical model
// routes to which of them.
//
// It lives outside internal/provider deliberately. That package is the
// vendor-neutral contract and must stay free of storage concerns; this one
// knows about PostgreSQL and about sealed credentials, and hands the other a
// finished registry (ARCHITECTURE-v1 sections 26 and 46).
package compute

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/provider/anthropic"
	"github.com/chiotron/ai-control-plane/internal/provider/ollama"
	"github.com/chiotron/ai-control-plane/internal/provider/openai"
	"github.com/chiotron/ai-control-plane/internal/secret"
)

var (
	ErrNotFound    = errors.New("provider not found")
	ErrUnknownKind = errors.New("unknown provider kind")
	ErrInUse       = errors.New("provider is still routed to")
)

// Kinds are the adapters that exist. The set is closed because an unknown kind
// has no code behind it: accepting one would store a row that fails on
// somebody's first request instead of at the moment the mistake was made.
const (
	KindOllama = "ollama"
	KindOpenAI = "openai-compatible"
	KindClaude = "anthropic"
	KindVLLM   = "vllm"
	KindNIM    = "nvidia-nim"
)

var Kinds = []string{KindOllama, KindOpenAI, KindClaude, KindVLLM, KindNIM}

// NeedsCredential reports whether a kind is useless without an API key. A local
// Ollama or private vLLM needs none, which is what lets a development deployment run with no
// encryption key configured at all.
func NeedsCredential(kind string) bool {
	return kind == KindOpenAI || kind == KindClaude || kind == KindNIM
}

// Provider is one configured model backend.
//
// The credential is never part of this struct: it is read only when an adapter
// is built, and no path exists from an HTTP handler to the plaintext.
type Provider struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Kind        string `json:"kind"`
	BaseURL     string `json:"baseUrl"`
	// HasCredential and CredentialHint let an operator tell two keys apart
	// without the platform ever handing one back.
	HasCredential  bool   `json:"hasCredential"`
	CredentialHint string `json:"credentialHint,omitempty"`
	// MaxClassification is the most sensitive content that may be sent here.
	MaxClassification string     `json:"maxClassification"`
	Enabled           bool       `json:"enabled"`
	TimeoutSeconds    int        `json:"timeoutSeconds"`
	CompanyID         string     `json:"companyId,omitempty"`
	LastCheckedAt     *time.Time `json:"lastCheckedAt,omitempty"`
	LastStatus        string     `json:"lastStatus,omitempty"`
	LastError         string     `json:"lastError,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

// Route maps a logical model id to a provider and an upstream model name.
type Route struct {
	ID            string    `json:"id"`
	Logical       string    `json:"logical"`
	ProviderSlug  string    `json:"provider"`
	UpstreamModel string    `json:"model"`
	IsDefault     bool      `json:"default"`
	Enabled       bool      `json:"enabled"`
	CompanyID     string    `json:"companyId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Store struct {
	pool   *pgxpool.Pool
	sealer *secret.Sealer
}

func NewStore(pool *pgxpool.Pool, sealer *secret.Sealer) *Store {
	return &Store{pool: pool, sealer: sealer}
}

const providerColumns = `
	p.id::text, p.slug, p.name, p.description, p.kind, p.base_url,
	p.credential IS NOT NULL, p.credential_hint, p.max_classification,
	p.enabled, p.timeout_seconds, coalesce(p.company_id, ''),
	p.last_checked_at, p.last_status, p.last_error, p.created_at, p.updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanProvider(row scanner) (Provider, error) {
	var record Provider
	err := row.Scan(&record.ID, &record.Slug, &record.Name, &record.Description,
		&record.Kind, &record.BaseURL, &record.HasCredential, &record.CredentialHint,
		&record.MaxClassification, &record.Enabled, &record.TimeoutSeconds, &record.CompanyID,
		&record.LastCheckedAt, &record.LastStatus, &record.LastError,
		&record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func (s *Store) ListProviders(ctx context.Context) ([]Provider, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+providerColumns+`
		FROM providers p WHERE p.deleted_at IS NULL ORDER BY p.slug`)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
	}
	defer rows.Close()

	var providers []Provider
	for rows.Next() {
		record, err := scanProvider(rows)
		if err != nil {
			return nil, fmt.Errorf("list providers: %w", err)
		}
		providers = append(providers, record)
	}
	return providers, rows.Err()
}

// CreateParams is a new provider. Credential is plaintext on the way in and is
// sealed before it reaches a disk.
type CreateParams struct {
	Slug              string
	Name              string
	Description       string
	Kind              string
	BaseURL           string
	Credential        string
	MaxClassification string
	TimeoutSeconds    int
	CompanyID         string
	CreatedBy         string
}

func (s *Store) CreateProvider(ctx context.Context, params CreateParams) (Provider, error) {
	if !slices.Contains(Kinds, params.Kind) {
		return Provider{}, fmt.Errorf("%w: %q (known kinds: %s)",
			ErrUnknownKind, params.Kind, strings.Join(Kinds, ", "))
	}
	if strings.TrimSpace(params.Slug) == "" || strings.TrimSpace(params.BaseURL) == "" {
		return Provider{}, fmt.Errorf("provider slug and base URL are required")
	}
	// A kind that needs a credential and has none would be created, listed as
	// enabled and fail on first use. Refusing here puts the error where the
	// mistake is.
	if NeedsCredential(params.Kind) && params.Credential == "" {
		return Provider{}, fmt.Errorf("provider kind %q requires an API credential", params.Kind)
	}

	sealed, hint, err := s.seal(params.Credential)
	if err != nil {
		return Provider{}, err
	}
	if params.TimeoutSeconds <= 0 {
		params.TimeoutSeconds = 60
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO providers (slug, name, description, kind, base_url, credential,
		                       credential_hint, max_classification, timeout_seconds,
		                       company_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, nullif($10, ''), $11)
		RETURNING `+strings.ReplaceAll(providerColumns, "p.", ""),
		params.Slug, params.Name, params.Description, params.Kind, params.BaseURL,
		sealed, hint, params.MaxClassification, params.TimeoutSeconds,
		params.CompanyID, params.CreatedBy)

	record, err := scanProvider(row)
	if err != nil {
		return Provider{}, fmt.Errorf("create provider: %w", err)
	}
	return record, nil
}

// UpdateParams carries only what an operator may change. A nil field is left
// alone, which is what lets the UI send a partial edit without having to
// re-supply a credential it was never allowed to read.
type UpdateParams struct {
	Name              *string
	Description       *string
	BaseURL           *string
	Credential        *string
	MaxClassification *string
	Enabled           *bool
	TimeoutSeconds    *int
}

func (s *Store) UpdateProvider(ctx context.Context, slug string, params UpdateParams) (Provider, error) {
	var sealed []byte
	var hint string
	if params.Credential != nil {
		var err error
		sealed, hint, err = s.seal(*params.Credential)
		if err != nil {
			return Provider{}, err
		}
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE providers p SET
			name = coalesce($2, p.name),
			description = coalesce($3, p.description),
			base_url = coalesce($4, p.base_url),
			credential = CASE WHEN $5::boolean THEN $6::bytea ELSE p.credential END,
			credential_hint = CASE WHEN $5::boolean THEN $7::text ELSE p.credential_hint END,
			max_classification = coalesce($8, p.max_classification),
			enabled = coalesce($9, p.enabled),
			timeout_seconds = coalesce($10, p.timeout_seconds),
			updated_at = now()
		WHERE p.slug = $1 AND p.deleted_at IS NULL
		RETURNING `+strings.ReplaceAll(providerColumns, "p.", ""),
		slug, params.Name, params.Description, params.BaseURL,
		params.Credential != nil, sealed, hint,
		params.MaxClassification, params.Enabled, params.TimeoutSeconds)

	record, err := scanProvider(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Provider{}, fmt.Errorf("%w: %q", ErrNotFound, slug)
	case err != nil:
		return Provider{}, fmt.Errorf("update provider: %w", err)
	}
	return record, nil
}

// DeleteProvider soft-deletes a provider, refusing while a route still points
// at it: removing it silently would turn every call on that route into an
// unknown-model error with nothing to explain why.
func (s *Store) DeleteProvider(ctx context.Context, slug string) error {
	var routed int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*) FROM model_routes
		WHERE provider_slug = $1 AND deleted_at IS NULL`, slug).Scan(&routed); err != nil {
		return fmt.Errorf("count routes: %w", err)
	}
	if routed > 0 {
		return fmt.Errorf("%w: %d route(s) still name %q", ErrInUse, routed, slug)
	}

	tag, err := s.pool.Exec(ctx, `
		UPDATE providers SET deleted_at = now(), updated_at = now()
		WHERE slug = $1 AND deleted_at IS NULL`, slug)
	if err != nil {
		return fmt.Errorf("delete provider: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, slug)
	}
	return nil
}

// RecordCheck stores the outcome of a connection test, so the providers page
// can show why a backend is not answering without an operator reading logs.
func (s *Store) RecordCheck(ctx context.Context, slug, status, failure string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE providers SET last_checked_at = now(), last_status = $2, last_error = $3
		WHERE slug = $1 AND deleted_at IS NULL`, slug, status, failure)
	if err != nil {
		return fmt.Errorf("record provider check: %w", err)
	}
	return nil
}

func (s *Store) seal(credential string) ([]byte, string, error) {
	if credential == "" {
		return nil, "", nil
	}
	if !s.sealer.Enabled() {
		// Storing it in the clear would be the convenient thing and the wrong one.
		return nil, "", fmt.Errorf(
			"%w: set CONFIG_ENCRYPTION_KEY before storing a provider credential", secret.ErrNoKey)
	}
	sealed, err := s.sealer.Seal(credential)
	if err != nil {
		return nil, "", fmt.Errorf("seal credential: %w", err)
	}
	return sealed, secret.Hint(credential), nil
}

const routeColumns = `
	id::text, logical, provider_slug, upstream_model, is_default, enabled,
	coalesce(company_id, ''), created_at, updated_at`

func scanRoute(row scanner) (Route, error) {
	var record Route
	err := row.Scan(&record.ID, &record.Logical, &record.ProviderSlug, &record.UpstreamModel,
		&record.IsDefault, &record.Enabled, &record.CompanyID, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func (s *Store) ListRoutes(ctx context.Context) ([]Route, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+routeColumns+`
		FROM model_routes WHERE deleted_at IS NULL ORDER BY logical`)
	if err != nil {
		return nil, fmt.Errorf("list routes: %w", err)
	}
	defer rows.Close()

	var routes []Route
	for rows.Next() {
		record, err := scanRoute(rows)
		if err != nil {
			return nil, fmt.Errorf("list routes: %w", err)
		}
		routes = append(routes, record)
	}
	return routes, rows.Err()
}

type RouteParams struct {
	Logical       string
	ProviderSlug  string
	UpstreamModel string
	IsDefault     bool
	CompanyID     string
	CreatedBy     string
}

// SaveRoute creates or replaces a route by logical id.
//
// Clearing any existing default happens in the same transaction as setting the
// new one, because the schema permits exactly one and doing it in two steps
// would fail on the unique index halfway.
func (s *Store) SaveRoute(ctx context.Context, params RouteParams) (Route, error) {
	if strings.TrimSpace(params.Logical) == "" || strings.TrimSpace(params.UpstreamModel) == "" {
		return Route{}, fmt.Errorf("a route needs a logical id and an upstream model")
	}

	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM providers WHERE slug = $1 AND deleted_at IS NULL)`,
		params.ProviderSlug).Scan(&exists); err != nil {
		return Route{}, fmt.Errorf("check provider: %w", err)
	}
	if !exists {
		return Route{}, fmt.Errorf("%w: %q", ErrNotFound, params.ProviderSlug)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Route{}, fmt.Errorf("begin save route: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if params.IsDefault {
		if _, err := tx.Exec(ctx, `
			UPDATE model_routes SET is_default = false, updated_at = now()
			WHERE is_default AND deleted_at IS NULL AND logical <> $1`, params.Logical); err != nil {
			return Route{}, fmt.Errorf("clear previous default: %w", err)
		}
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO model_routes (logical, provider_slug, upstream_model, is_default, company_id, created_by)
		VALUES ($1, $2, $3, $4, nullif($5, ''), $6)
		ON CONFLICT (logical) WHERE deleted_at IS NULL DO UPDATE SET
			provider_slug = excluded.provider_slug,
			upstream_model = excluded.upstream_model,
			is_default = excluded.is_default,
			updated_at = now()
		RETURNING `+routeColumns,
		params.Logical, params.ProviderSlug, params.UpstreamModel,
		params.IsDefault, params.CompanyID, params.CreatedBy)

	record, err := scanRoute(row)
	if err != nil {
		return Route{}, fmt.Errorf("save route: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Route{}, fmt.Errorf("commit save route: %w", err)
	}
	return record, nil
}

// DeleteRoute removes a route, refusing to remove the default one: a platform
// with no default has no answer for a caller who names no model.
func (s *Store) DeleteRoute(ctx context.Context, logical string) error {
	var isDefault bool
	err := s.pool.QueryRow(ctx, `
		SELECT is_default FROM model_routes
		WHERE logical = $1 AND deleted_at IS NULL`, logical).Scan(&isDefault)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %q", ErrNotFound, logical)
	case err != nil:
		return fmt.Errorf("read route: %w", err)
	}
	if isDefault {
		return fmt.Errorf("%w: %q is the default route; make another route the default first",
			ErrInUse, logical)
	}

	if _, err := s.pool.Exec(ctx, `
		UPDATE model_routes SET deleted_at = now(), updated_at = now()
		WHERE logical = $1 AND deleted_at IS NULL`, logical); err != nil {
		return fmt.Errorf("delete route: %w", err)
	}
	return nil
}

// Adapter builds the client for one provider row.
//
// It is exported so a connection test can reach a provider that is not in the
// live registry yet — an operator has to be able to check a credential before
// routing traffic through it.
func (s *Store) Adapter(ctx context.Context, record Provider) (provider.LLM, error) {
	credential, err := s.credential(ctx, record.Slug)
	if err != nil {
		return nil, err
	}
	timeout := time.Duration(record.TimeoutSeconds) * time.Second

	switch record.Kind {
	case KindOllama:
		return ollama.New(record.BaseURL, timeout), nil
	case KindOpenAI, KindVLLM, KindNIM:
		return openai.New(record.Slug, record.BaseURL, credential, timeout), nil
	case KindClaude:
		return anthropic.New(record.Slug, record.BaseURL, credential, timeout), nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownKind, record.Kind)
	}
}

func (s *Store) credential(ctx context.Context, slug string) (string, error) {
	var sealed []byte
	err := s.pool.QueryRow(ctx, `
		SELECT credential FROM providers WHERE slug = $1 AND deleted_at IS NULL`, slug).Scan(&sealed)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", fmt.Errorf("%w: %q", ErrNotFound, slug)
	case err != nil:
		return "", fmt.Errorf("read credential: %w", err)
	}
	if len(sealed) == 0 {
		return "", nil
	}
	return s.sealer.Open(sealed)
}
