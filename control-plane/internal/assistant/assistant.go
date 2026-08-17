// Package assistant owns the assistant catalogue.
//
// An assistant names the logical model and the instructions to use, so the
// portal selects an assistant rather than a model and provider details stay
// hidden from the user (ARCHITECTURE-v1 section 10).
package assistant

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("assistant not found")

type Assistant struct {
	ID           string   `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Instructions string   `json:"instructions,omitempty"`
	LogicalModel string   `json:"logicalModel"`
	Temperature  *float64 `json:"temperature,omitempty"`
	MaxTokens    *int     `json:"maxTokens,omitempty"`
	CompanyID    string   `json:"companyId,omitempty"`
	Enabled      bool     `json:"enabled"`

	// Retrieval is assistant policy: off, auto or always. MaxSteps bounds the
	// agent's plan for this assistant, within the platform budget.
	Retrieval string `json:"retrieval"`
	MaxSteps  int    `json:"maxSteps"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Public strips assistant policy from a record. Instructions are configuration
// the operator wrote, not something every caller needs to read back.
func (a Assistant) Public() Assistant {
	a.Instructions = ""
	return a
}

type CreateParams struct {
	Slug         string
	Name         string
	Description  string
	Instructions string
	LogicalModel string
	Temperature  *float64
	MaxTokens    *int
	CompanyID    string
	Retrieval    string
	MaxSteps     int
	CreatedBy    string
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const columns = `id::text, slug, name, description, instructions, logical_model,
	temperature, max_tokens, coalesce(company_id, ''), enabled, retrieval, max_steps,
	created_at, updated_at`

// List returns the assistants a caller may use.
//
// The company predicate is applied in SQL rather than after the fact, so an
// assistant belonging to another company never reaches the process
// (ARCHITECTURE-v1 section 5). An assistant with no company is platform-wide.
func (s *Store) List(ctx context.Context, companyID string) ([]Assistant, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+columns+`
		FROM assistants
		WHERE deleted_at IS NULL
		  AND enabled
		  AND (company_id IS NULL OR company_id = nullif($1, ''))
		ORDER BY name`, companyID)
	if err != nil {
		return nil, fmt.Errorf("list assistants: %w", err)
	}
	defer rows.Close()

	var assistants []Assistant
	for rows.Next() {
		record, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("list assistants: %w", err)
		}
		assistants = append(assistants, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list assistants: %w", err)
	}
	return assistants, nil
}

// Resolve looks an assistant up by slug or id, subject to the same company
// predicate as List so an id cannot be used to reach another company's record.
func (s *Store) Resolve(ctx context.Context, reference, companyID string) (Assistant, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return Assistant{}, fmt.Errorf("%w: no assistant named", ErrNotFound)
	}

	// The cast is guarded so a slug that is not a UUID does not error the query.
	row := s.pool.QueryRow(ctx, `
		SELECT `+columns+`
		FROM assistants
		WHERE deleted_at IS NULL
		  AND enabled
		  AND (company_id IS NULL OR company_id = nullif($2, ''))
		  AND (slug = $1 OR ($1 ~ '^[0-9a-fA-F-]{36}$' AND id = $1::uuid))`, reference, companyID)

	record, err := scan(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Assistant{}, fmt.Errorf("%w: %q", ErrNotFound, reference)
	case err != nil:
		return Assistant{}, fmt.Errorf("resolve assistant %q: %w", reference, err)
	}
	return record, nil
}

func (s *Store) Create(ctx context.Context, params CreateParams) (Assistant, error) {
	if strings.TrimSpace(params.Slug) == "" || strings.TrimSpace(params.Name) == "" {
		return Assistant{}, fmt.Errorf("assistant slug and name are required")
	}
	if strings.TrimSpace(params.LogicalModel) == "" {
		return Assistant{}, fmt.Errorf("assistant logicalModel is required")
	}

	retrieval := params.Retrieval
	if retrieval == "" {
		// The column default carries the platform's own baseline.
		retrieval = "auto"
	}
	maxSteps := params.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 3
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO assistants (slug, name, description, instructions, logical_model,
		                        temperature, max_tokens, company_id, retrieval, max_steps, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, nullif($8, ''), $9, $10, $11)
		RETURNING `+columns,
		params.Slug, params.Name, params.Description, params.Instructions, params.LogicalModel,
		params.Temperature, params.MaxTokens, params.CompanyID, retrieval, maxSteps, params.CreatedBy)

	record, err := scan(row)
	if err != nil {
		return Assistant{}, fmt.Errorf("create assistant: %w", err)
	}
	return record, nil
}

type scannable interface{ Scan(dest ...any) error }

func scan(row scannable) (Assistant, error) {
	var record Assistant
	err := row.Scan(&record.ID, &record.Slug, &record.Name, &record.Description, &record.Instructions,
		&record.LogicalModel, &record.Temperature, &record.MaxTokens, &record.CompanyID,
		&record.Enabled, &record.Retrieval, &record.MaxSteps, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}
