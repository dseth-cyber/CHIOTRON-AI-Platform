// Package favorite records what a caller has marked to come back to.
//
// Favourites are per credential, like conversations: one key never sees
// another's. When the Identity Service arrives and a person owns several
// credentials, the actor becomes the person and nothing else here changes.
package favorite

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUnknownKind = errors.New("unknown favourite kind")

const (
	KindAssistant    = "assistant"
	KindConversation = "conversation"
	KindDocument     = "document"
)

// Kinds is what may be favourited. An unknown kind is refused rather than
// stored: a typo would otherwise create a favourite nothing can ever resolve.
var Kinds = []string{KindAssistant, KindConversation, KindDocument}

// Favorite is a mark, resolved against the record it points at.
type Favorite struct {
	Kind      string    `json:"kind"`
	TargetID  string    `json:"targetId"`
	Label     string    `json:"label"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func Normalise(kind string) (string, error) {
	canonical := strings.ToLower(strings.TrimSpace(kind))
	if !slices.Contains(Kinds, canonical) {
		return "", fmt.Errorf("%w: %q (known kinds: %s)", ErrUnknownKind, kind, strings.Join(Kinds, ", "))
	}
	return canonical, nil
}

// Add is idempotent, so a client that cannot tell whether a mark was already
// stored can simply set it.
func (s *Store) Add(ctx context.Context, actorID, companyID, kind, targetID string) error {
	canonical, err := Normalise(kind)
	if err != nil {
		return err
	}
	if strings.TrimSpace(targetID) == "" {
		return fmt.Errorf("favourite target is required")
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO favorites (actor_id, company_id, kind, target_id)
		VALUES ($1, nullif($2, ''), $3, $4)
		ON CONFLICT (actor_id, kind, target_id) DO NOTHING`,
		actorID, companyID, canonical, targetID)
	if err != nil {
		return fmt.Errorf("add favourite: %w", err)
	}
	return nil
}

func (s *Store) Remove(ctx context.Context, actorID, kind, targetID string) error {
	canonical, err := Normalise(kind)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		DELETE FROM favorites WHERE actor_id = $1 AND kind = $2 AND target_id = $3`,
		actorID, canonical, targetID)
	if err != nil {
		return fmt.Errorf("remove favourite: %w", err)
	}
	return nil
}

// List resolves each mark against its record and drops the ones whose target is
// gone or is no longer readable.
//
// The joins repeat the same access predicates the owning endpoints use. A
// favourite must not become a way to read the title of a document whose
// clearance has since been raised.
func (s *Store) List(ctx context.Context, actorID, companyID string, classifications []string) ([]Favorite, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT f.kind, f.target_id, f.created_at,
		       coalesce(a.name, c.title, d.title, '') AS label,
		       -- The empty-string fallback must come last: coalesce stops at the
		       -- first non-null, and '' is not null.
		       coalesce(a.description, d.classification, '') AS detail
		FROM favorites f
		LEFT JOIN assistants a
		       ON f.kind = 'assistant' AND a.id::text = f.target_id
		      AND a.deleted_at IS NULL AND a.enabled
		      AND (a.company_id IS NULL OR a.company_id = nullif($2, ''))
		LEFT JOIN conversations c
		       ON f.kind = 'conversation' AND c.id::text = f.target_id
		      AND c.deleted_at IS NULL AND c.actor_id = $1
		LEFT JOIN documents d
		       ON f.kind = 'document' AND d.id::text = f.target_id
		      AND d.deleted_at IS NULL
		      AND (d.company_id IS NULL OR d.company_id = nullif($2, ''))
		      AND d.classification = ANY($3)
		WHERE f.actor_id = $1
		  AND coalesce(a.id::text, c.id::text, d.id::text) IS NOT NULL
		ORDER BY f.created_at DESC`,
		actorID, companyID, classifications)
	if err != nil {
		return nil, fmt.Errorf("list favourites: %w", err)
	}
	defer rows.Close()

	var favourites []Favorite
	for rows.Next() {
		var record Favorite
		if err := rows.Scan(&record.Kind, &record.TargetID, &record.CreatedAt,
			&record.Label, &record.Detail); err != nil {
			return nil, fmt.Errorf("list favourites: %w", err)
		}
		favourites = append(favourites, record)
	}
	return favourites, rows.Err()
}

// IDs returns the favourited target ids of one kind, for a client that wants to
// mark items in a list without fetching each one.
func (s *Store) IDs(ctx context.Context, actorID, kind string) ([]string, error) {
	canonical, err := Normalise(kind)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT target_id FROM favorites WHERE actor_id = $1 AND kind = $2`, actorID, canonical)
	if err != nil {
		return nil, fmt.Errorf("list favourite ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("list favourite ids: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
