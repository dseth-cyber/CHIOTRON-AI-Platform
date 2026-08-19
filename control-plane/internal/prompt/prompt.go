// Package prompt owns the prompt template registry.
package prompt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("prompt template not found")

type Template struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Template    string    `json:"template"`
	Variables   []string  `json:"variables"`
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type CreateParams struct {
	Slug        string
	Name        string
	Description string
	Template    string
	Variables   []string
	CreatedBy   string
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

const columns = `id::text, slug, name, description, template, variables, created_by, created_at, updated_at`

// List returns all active prompt templates.
func (s *Store) List(ctx context.Context) ([]Template, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+columns+`
		FROM prompt_templates
		WHERE deleted_at IS NULL
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer rows.Close()

	var templates []Template
	for rows.Next() {
		tpl, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("list prompt templates: %w", err)
		}
		templates = append(templates, tpl)
	}
	return templates, rows.Err()
}

// GetBySlug retrieves a prompt template by its slug.
func (s *Store) GetBySlug(ctx context.Context, slug string) (Template, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+columns+`
		FROM prompt_templates
		WHERE slug = $1 AND deleted_at IS NULL`, slug)
	if err != nil {
		return Template{}, fmt.Errorf("get prompt template %q: %w", slug, err)
	}
	defer rows.Close()

	if !rows.Next() {
		return Template{}, ErrNotFound
	}
	return scan(rows)
}

// Create inserts a new prompt template.
func (s *Store) Create(ctx context.Context, params CreateParams) (Template, error) {
	if params.Slug == "" || params.Name == "" || params.Template == "" {
		return Template{}, errors.New("slug, name and template are required")
	}
	if params.Variables == nil {
		params.Variables = []string{}
	}
	varsJSON, err := json.Marshal(params.Variables)
	if err != nil {
		return Template{}, fmt.Errorf("marshal variables: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		INSERT INTO prompt_templates (slug, name, description, template, variables, created_by)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6)
		RETURNING `+columns,
		params.Slug, params.Name, params.Description, params.Template, string(varsJSON), params.CreatedBy)
	if err != nil {
		return Template{}, fmt.Errorf("create prompt template: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return Template{}, errors.New("insert returned no rows")
	}
	return scan(rows)
}

// Delete soft-deletes a prompt template.
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE prompt_templates
		SET deleted_at = now()
		WHERE id = $1::uuid AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete prompt template %q: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scan(rows pgx.Rows) (Template, error) {
	var tpl Template
	var varsJSON []byte
	err := rows.Scan(
		&tpl.ID, &tpl.Slug, &tpl.Name, &tpl.Description, &tpl.Template,
		&varsJSON, &tpl.CreatedBy, &tpl.CreatedAt, &tpl.UpdatedAt,
	)
	if err != nil {
		return Template{}, err
	}
	if len(varsJSON) > 0 {
		if err := json.Unmarshal(varsJSON, &tpl.Variables); err != nil {
			tpl.Variables = []string{}
		}
	} else {
		tpl.Variables = []string{}
	}
	return tpl, nil
}
