package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/tool"
)

var ErrRunNotFound = errors.New("agent run not found")

// Store persists run traces and the tool-call audit trail.
type Store struct {
	pool *pgxpool.Pool
	// persistQuestion follows the prompt-logging policy: the question is user
	// content, so a run is recorded either way but its text may be withheld.
	persistQuestion bool
}

func NewStore(pool *pgxpool.Pool, persistQuestion bool) *Store {
	return &Store{pool: pool, persistQuestion: persistQuestion}
}

// RunSummary is a stored run as a caller reads it back.
type RunSummary struct {
	ID            string    `json:"id"`
	AssistantSlug string    `json:"assistantSlug,omitempty"`
	Question      string    `json:"question,omitempty"`
	Redacted      bool      `json:"questionRedacted,omitempty"`
	Status        string    `json:"status"`
	StepCount     int       `json:"stepCount"`
	CitationCount int       `json:"citationCount"`
	Conflicted    bool      `json:"conflicted"`
	TotalTokens   int       `json:"totalTokens"`
	LatencyMs     int64     `json:"latencyMs"`
	Error         string    `json:"error,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// Save writes the run and its steps in one transaction, so a trace is never
// half-recorded against an answer that was delivered whole.
func (s *Store) Save(ctx context.Context, caller auth.Identity, assistantID, conversationID,
	question, status, failure string, answer Answer) (string, error) {

	stored := question
	if !s.persistQuestion {
		stored = ""
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin save run: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	var runID string
	err = tx.QueryRow(ctx, `
		INSERT INTO agent_runs (actor_id, api_key_id, company_id, assistant_id, conversation_id,
		                        question, question_redacted, status, step_count, citation_count,
		                        conflicted, total_tokens, latency_ms, error, trace_id)
		VALUES ($1, nullif($2, '')::uuid, nullif($3, ''), nullif($4, '')::uuid, nullif($5, '')::uuid,
		        $6, $7, $8, $9, $10, $11, $12, $13, nullif($14, ''), nullif($15, ''))
		RETURNING id::text`,
		caller.KeyID, caller.KeyID, caller.CompanyID, assistantID, conversationID,
		stored, !s.persistQuestion, status, len(answer.Steps), len(answer.Citations),
		answer.Conflicted, answer.Usage.TotalTokens, answer.LatencyMs, failure, traceID(ctx)).Scan(&runID)
	if err != nil {
		return "", fmt.Errorf("insert agent run: %w", err)
	}

	for _, step := range answer.Steps {
		detail, err := json.Marshal(orEmpty(step.Detail))
		if err != nil {
			return "", fmt.Errorf("encode step detail: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO agent_steps (run_id, ordinal, kind, summary, detail, outcome, latency_ms)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
			runID, step.Ordinal, step.Kind, step.Summary, detail, step.Outcome, step.Millis); err != nil {
			return "", fmt.Errorf("insert agent step %d: %w", step.Ordinal, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit save run: %w", err)
	}
	return runID, nil
}

// Get returns a run and its steps, scoped to the caller that produced it.
func (s *Store) Get(ctx context.Context, id, actorID string) (RunSummary, []Step, error) {
	if !isUUID(id) {
		return RunSummary{}, nil, fmt.Errorf("%w: %q", ErrRunNotFound, id)
	}

	var summary RunSummary
	err := s.pool.QueryRow(ctx, `
		SELECT r.id::text, coalesce(a.slug, ''), r.question, r.question_redacted, r.status,
		       r.step_count, r.citation_count, r.conflicted, r.total_tokens, r.latency_ms,
		       coalesce(r.error, ''), r.created_at
		FROM agent_runs r LEFT JOIN assistants a ON a.id = r.assistant_id
		WHERE r.id = $1::uuid AND r.actor_id = $2`, id, actorID).
		Scan(&summary.ID, &summary.AssistantSlug, &summary.Question, &summary.Redacted, &summary.Status,
			&summary.StepCount, &summary.CitationCount, &summary.Conflicted, &summary.TotalTokens,
			&summary.LatencyMs, &summary.Error, &summary.CreatedAt)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return RunSummary{}, nil, fmt.Errorf("%w: %q", ErrRunNotFound, id)
	case err != nil:
		return RunSummary{}, nil, fmt.Errorf("read agent run: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		SELECT ordinal, kind, summary, detail, outcome, latency_ms
		FROM agent_steps WHERE run_id = $1::uuid ORDER BY ordinal`, id)
	if err != nil {
		return RunSummary{}, nil, fmt.Errorf("read agent steps: %w", err)
	}
	defer rows.Close()

	var steps []Step
	for rows.Next() {
		var step Step
		var detail []byte
		if err := rows.Scan(&step.Ordinal, &step.Kind, &step.Summary, &detail, &step.Outcome, &step.Millis); err != nil {
			return RunSummary{}, nil, fmt.Errorf("read agent steps: %w", err)
		}
		if len(detail) > 0 {
			_ = json.Unmarshal(detail, &step.Detail)
		}
		steps = append(steps, step)
	}
	return summary, steps, rows.Err()
}

// RecordToolCall implements tool.Recorder. A failure to record is logged by the
// caller rather than failing the tool call it describes.
func (s *Store) RecordToolCall(ctx context.Context, call tool.Record) {
	arguments, err := json.Marshal(orEmptyAny(call.Arguments))
	if err != nil {
		arguments = []byte(`{}`)
	}
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO tool_calls (run_id, tool_slug, actor_id, api_key_id, company_id,
		                        arguments, outcome, error, latency_ms, trace_id)
		VALUES (nullif($1, '')::uuid, $2, $3, nullif($4, '')::uuid, nullif($5, ''),
		        $6, $7, nullif($8, ''), $9, nullif($10, ''))`,
		call.RunID, call.Slug, call.Caller.KeyID, call.Caller.KeyID, call.Caller.CompanyID,
		arguments, call.Outcome, call.Error, call.Latency.Milliseconds(), traceID(ctx))
}

// Tools reads the registry configuration.
func (s *Store) Tools(ctx context.Context) ([]tool.Registration, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT slug, name, description, kind, required_scope, coalesce(company_id, ''), max_calls_per_minute
		FROM tools WHERE deleted_at IS NULL AND enabled ORDER BY slug`)
	if err != nil {
		return nil, fmt.Errorf("read tool registry: %w", err)
	}
	defer rows.Close()

	var registrations []tool.Registration
	for rows.Next() {
		var registration tool.Registration
		if err := rows.Scan(&registration.Slug, &registration.Name, &registration.Description,
			&registration.Kind, &registration.RequiredScope, &registration.CompanyID,
			&registration.MaxCallsPerMinute); err != nil {
			return nil, fmt.Errorf("read tool registry: %w", err)
		}
		registrations = append(registrations, registration)
	}
	return registrations, rows.Err()
}

func traceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}

func orEmpty(detail map[string]any) map[string]any {
	if detail == nil {
		return map[string]any{}
	}
	return detail
}

func orEmptyAny(arguments map[string]any) map[string]any {
	if arguments == nil {
		return map[string]any{}
	}
	return arguments
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
