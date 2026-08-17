// Package audit records what happened and what it cost.
//
// Every AI request, tool call and configuration change creates usage metadata
// and an audit event (ARCHITECTURE-v1 section 5). Rows are written with
// published_at NULL and drained to the `ai.audit.v1` and `ai.usage.v1` topics
// once Kafka is available (section 7); until then the outbox is the durable
// record and nothing is lost by the broker being absent.
package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/trace"
)

const (
	OutcomeSuccess = "success"
	OutcomeDenied  = "denied"
	OutcomeFailure = "failure"
)

// Event is one auditable action.
type Event struct {
	ActorID      string
	APIKeyID     string
	CompanyID    string
	Action       string
	ResourceType string
	ResourceID   string
	Outcome      string
	Metadata     map[string]any
}

// Usage is what one model call consumed.
type Usage struct {
	ActorID          string
	APIKeyID         string
	CompanyID        string
	LogicalModel     string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	LatencyMs        int64
	Outcome          string
}

type Recorder struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

func NewRecorder(pool *pgxpool.Pool, log *slog.Logger) *Recorder {
	return &Recorder{pool: pool, log: log}
}

// Record writes an audit row. A failure here must not fail the caller's
// request, so it is logged at error level and swallowed: losing the action is
// worse than losing its audit line, and the log still preserves the evidence.
func (r *Recorder) Record(ctx context.Context, event Event) {
	metadata, err := json.Marshal(orEmpty(event.Metadata))
	if err != nil {
		r.log.Error("encode audit metadata", "action", event.Action, "error", err)
		return
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_id, action, resource_type, resource_id,
		                        metadata, company_id, api_key_id, trace_id, outcome)
		VALUES ($1, $2, $3, nullif($4, ''), $5, nullif($6, ''), nullif($7, '')::uuid, nullif($8, ''), $9)`,
		event.ActorID, event.Action, event.ResourceType, event.ResourceID,
		metadata, event.CompanyID, event.APIKeyID, traceID(ctx), orDefault(event.Outcome, OutcomeSuccess))
	if err != nil {
		r.log.Error("write audit log", "action", event.Action, "error", err)
	}
}

// RecordUsage writes a usage row for one model call.
func (r *Recorder) RecordUsage(ctx context.Context, usage Usage) {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO usage_events (actor_id, api_key_id, company_id, logical_model, provider, model,
		                          prompt_tokens, completion_tokens, total_tokens, latency_ms, outcome, trace_id)
		VALUES ($1, nullif($2, '')::uuid, nullif($3, ''), $4, $5, $6, $7, $8, $9, $10, $11, nullif($12, ''))`,
		usage.ActorID, usage.APIKeyID, usage.CompanyID, usage.LogicalModel, usage.Provider, usage.Model,
		usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens, usage.LatencyMs,
		orDefault(usage.Outcome, OutcomeSuccess), traceID(ctx))
	if err != nil {
		r.log.Error("write usage event", "logicalModel", usage.LogicalModel, "error", err)
	}
}

// PendingCounts reports how much the outbox is holding, so an operator can see
// the backlog before a publisher exists.
func (r *Recorder) PendingCounts(ctx context.Context) (auditRows, usageRows int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM audit_logs WHERE published_at IS NULL),
		       (SELECT count(*) FROM usage_events WHERE published_at IS NULL)`).
		Scan(&auditRows, &usageRows)
	if err != nil {
		return 0, 0, fmt.Errorf("read outbox backlog: %w", err)
	}
	return auditRows, usageRows, nil
}

// traceID links an audit row to the trace that produced it.
func traceID(ctx context.Context) string {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}

func orEmpty(metadata map[string]any) map[string]any {
	if metadata == nil {
		return map[string]any{}
	}
	return metadata
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
