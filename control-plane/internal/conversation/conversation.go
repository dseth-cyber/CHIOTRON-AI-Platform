// Package conversation stores chat transcripts.
//
// Every read is scoped to the credential that created the conversation: one key
// can never see another's transcript. Retention and prompt logging are policy
// settings, not properties of this code (ARCHITECTURE-v1 section 5).
package conversation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("conversation not found")

// titleLength bounds the title derived from the opening question.
const titleLength = 80

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

type Conversation struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	AssistantID   string    `json:"assistantId,omitempty"`
	AssistantSlug string    `json:"assistantSlug,omitempty"`
	AssistantName string    `json:"assistantName,omitempty"`
	MessageCount  int       `json:"messageCount"`
	TotalTokens   int       `json:"totalTokens"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type Message struct {
	Role             string    `json:"role"`
	Content          string    `json:"content"`
	Redacted         bool      `json:"redacted,omitempty"`
	PromptTokens     int       `json:"promptTokens,omitempty"`
	CompletionTokens int       `json:"completionTokens,omitempty"`
	Model            string    `json:"model,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
}

// Turn is one exchange: what the caller asked and what the model answered.
type Turn struct {
	Question         string
	Answer           string
	PromptTokens     int
	CompletionTokens int
	Model            string
}

type Owner struct {
	ActorID   string
	APIKeyID  string
	CompanyID string
}

type Store struct {
	pool *pgxpool.Pool
	// persistContent carries the prompt-logging policy. When it is off the turn
	// is still recorded, without its text.
	persistContent bool
}

func NewStore(pool *pgxpool.Pool, persistContent bool) *Store {
	return &Store{pool: pool, persistContent: persistContent}
}

const listColumns = `c.id::text, c.title, coalesce(c.assistant_id::text, ''),
	coalesce(a.slug, ''), coalesce(a.name, ''), c.message_count, c.total_tokens,
	c.created_at, c.updated_at`

func (s *Store) Create(ctx context.Context, owner Owner, assistantID string) (Conversation, error) {
	row := s.pool.QueryRow(ctx, `
		WITH created AS (
			INSERT INTO conversations (actor_id, api_key_id, company_id, assistant_id)
			VALUES ($1, nullif($2, '')::uuid, nullif($3, ''), nullif($4, '')::uuid)
			RETURNING *
		)
		SELECT `+listColumns+`
		FROM created c LEFT JOIN assistants a ON a.id = c.assistant_id`,
		owner.ActorID, owner.APIKeyID, owner.CompanyID, assistantID)

	record, err := scan(row)
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return record, nil
}

func (s *Store) List(ctx context.Context, actorID string, limit int) ([]Conversation, error) {
	return s.list(ctx, actorID, limit, false)
}

// ListDeleted returns what the caller has thrown away but not yet purged, so a
// mistaken delete can be undone (ARCHITECTURE-v1 section 8).
func (s *Store) ListDeleted(ctx context.Context, actorID string, limit int) ([]Conversation, error) {
	return s.list(ctx, actorID, limit, true)
}

func (s *Store) list(ctx context.Context, actorID string, limit int, deleted bool) ([]Conversation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// The predicate is chosen here rather than interpolated, so the two listings
	// cannot drift apart or accidentally return both sets at once.
	predicate := "c.deleted_at IS NULL"
	ordering := "c.updated_at DESC"
	if deleted {
		predicate = "c.deleted_at IS NOT NULL"
		// The trash reads most-recently-discarded first: that is the one somebody
		// arriving here is most likely looking for.
		ordering = "c.deleted_at DESC"
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+listColumns+`
		FROM conversations c LEFT JOIN assistants a ON a.id = c.assistant_id
		WHERE c.actor_id = $1 AND `+predicate+`
		ORDER BY `+ordering+`
		LIMIT $2`, actorID, limit)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	var conversations []Conversation
	for rows.Next() {
		record, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("list conversations: %w", err)
		}
		conversations = append(conversations, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	return conversations, nil
}

// Get returns a conversation only to the caller that owns it. An id belonging to
// somebody else is reported as missing rather than forbidden, so an id cannot be
// probed for existence.
func (s *Store) Get(ctx context.Context, id, actorID string) (Conversation, error) {
	if !isUUID(id) {
		return Conversation{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	row := s.pool.QueryRow(ctx, `
		SELECT `+listColumns+`
		FROM conversations c LEFT JOIN assistants a ON a.id = c.assistant_id
		WHERE c.id = $1::uuid AND c.actor_id = $2 AND c.deleted_at IS NULL`, id, actorID)

	record, err := scan(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Conversation{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	case err != nil:
		return Conversation{}, fmt.Errorf("read conversation: %w", err)
	}
	return record, nil
}

func (s *Store) Messages(ctx context.Context, id, actorID string) ([]Message, error) {
	if _, err := s.Get(ctx, id, actorID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT role, content, content_redacted, prompt_tokens, completion_tokens,
		       coalesce(model, ''), created_at
		FROM messages WHERE conversation_id = $1::uuid ORDER BY id`, id)
	if err != nil {
		return nil, fmt.Errorf("read messages: %w", err)
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.Role, &message.Content, &message.Redacted,
			&message.PromptTokens, &message.CompletionTokens, &message.Model, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("read messages: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read messages: %w", err)
	}
	return messages, nil
}

// AppendTurn records one exchange and refreshes the conversation counters in a
// single transaction, so a transcript never gains a question without its answer.
func (s *Store) AppendTurn(ctx context.Context, id, actorID string, turn Turn) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin append turn: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	// Re-check ownership inside the transaction rather than trusting the caller.
	var currentTitle string
	err = tx.QueryRow(ctx, `
		SELECT title FROM conversations
		WHERE id = $1::uuid AND actor_id = $2 AND deleted_at IS NULL
		FOR UPDATE`, id, actorID).Scan(&currentTitle)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	case err != nil:
		return fmt.Errorf("lock conversation: %w", err)
	}

	question, answer := turn.Question, turn.Answer
	if !s.persistContent {
		question, answer = "", ""
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO messages (conversation_id, role, content, content_redacted, prompt_tokens, model)
		VALUES ($1::uuid, $2, $3, $4, $5, nullif($6, ''))`,
		id, RoleUser, question, !s.persistContent, turn.PromptTokens, turn.Model); err != nil {
		return fmt.Errorf("record question: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO messages (conversation_id, role, content, content_redacted, completion_tokens, model)
		VALUES ($1::uuid, $2, $3, $4, $5, nullif($6, ''))`,
		id, RoleAssistant, answer, !s.persistContent, turn.CompletionTokens, turn.Model); err != nil {
		return fmt.Errorf("record answer: %w", err)
	}

	// The title comes from the opening question, so history is readable without
	// asking a model to summarise it.
	title := currentTitle
	if title == "" {
		title = deriveTitle(turn.Question)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE conversations
		SET message_count = message_count + 2,
		    total_tokens = total_tokens + $2,
		    title = $3,
		    updated_at = now()
		WHERE id = $1::uuid`, id, turn.PromptTokens+turn.CompletionTokens, title); err != nil {
		return fmt.Errorf("update conversation counters: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit append turn: %w", err)
	}
	return nil
}

// Delete soft-deletes a conversation. Normal records use soft deletion so an
// audit trail keeps its references (ARCHITECTURE-v1 section 8).
func (s *Store) Delete(ctx context.Context, id, actorID string) error {
	if !isUUID(id) {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE conversations SET deleted_at = now(), updated_at = now()
		WHERE id = $1::uuid AND actor_id = $2 AND deleted_at IS NULL`, id, actorID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	return nil
}

// Restore returns a soft-deleted conversation to the caller's history.
//
// It matches only rows that are actually deleted, so restoring something already
// live reads as missing rather than silently doing nothing.
func (s *Store) Restore(ctx context.Context, id, actorID string) error {
	if !isUUID(id) {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE conversations SET deleted_at = NULL, updated_at = now()
		WHERE id = $1::uuid AND actor_id = $2 AND deleted_at IS NOT NULL`, id, actorID)
	if err != nil {
		return fmt.Errorf("restore conversation: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	return nil
}

// PersistsContent reports whether message text is being stored, so a caller can
// tell an empty transcript from a redacted one.
func (s *Store) PersistsContent() bool { return s.persistContent }

func deriveTitle(question string) string {
	title := strings.Join(strings.Fields(question), " ")
	if title == "" {
		return "Untitled conversation"
	}
	if len(title) <= titleLength {
		return title
	}
	// Cut on a rune boundary so a multi-byte character is not split in half.
	runes := []rune(title)
	if len(runes) <= titleLength {
		return title
	}
	return strings.TrimSpace(string(runes[:titleLength])) + "…"
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			isHex := (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

type scannable interface{ Scan(dest ...any) error }

func scan(row scannable) (Conversation, error) {
	var record Conversation
	err := row.Scan(&record.ID, &record.Title, &record.AssistantID, &record.AssistantSlug,
		&record.AssistantName, &record.MessageCount, &record.TotalTokens,
		&record.CreatedAt, &record.UpdatedAt)
	return record, err
}
