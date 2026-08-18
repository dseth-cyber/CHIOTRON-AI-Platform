package telemetry

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// meterName is the instrumentation scope. It appears on every series as
// otel_scope_name, which is how a dashboard tells platform metrics from the
// runtime and HTTP metrics that share the endpoint.
const meterName = "github.com/chiotron/ai-control-plane"

// Metrics is the platform's own metric contract.
//
// The names are the contract: dashboards and alert rules are written against
// them, so they change only with the dashboards that read them
// (ARCHITECTURE-v1 section 9). Everything here is emitted through the same
// Prometheus registry the runtime metrics use, so there is one scrape endpoint.
type Metrics struct {
	chatRequests   metric.Int64Counter
	chatTokens     metric.Int64Counter
	chatLatency    metric.Float64Histogram
	chatCost       metric.Float64Counter
	agentRuns      metric.Int64Counter
	agentSteps     metric.Int64Counter
	retrievalScore metric.Float64Histogram
	toolCalls      metric.Int64Counter
	toolLatency    metric.Float64Histogram
	documents      metric.Int64Counter
	chunks         metric.Int64Counter
	authFailures   metric.Int64Counter
	rateLimits     metric.Int64Counter

	// prices are configured rates per 1000 tokens, keyed by logical model. Local
	// inference has no invoice, so cost is what an operator declares a token is
	// worth, not something the platform can discover.
	prices   map[string]float64
	currency string
	log      *slog.Logger
}

// NewMetrics builds the instruments. A failure to create one is not fatal: losing
// a metric must not stop the platform serving requests.
func NewMetrics(prices map[string]float64, currency string, log *slog.Logger) *Metrics {
	meter := otel.Meter(meterName)
	metrics := &Metrics{prices: prices, currency: currency, log: log}

	var err error
	if metrics.chatRequests, err = meter.Int64Counter("ai_chat_requests",
		metric.WithDescription("Completions requested, by logical model and outcome.")); err != nil {
		log.Error("create metric", "name", "ai_chat_requests", "error", err)
	}
	if metrics.chatTokens, err = meter.Int64Counter("ai_chat_tokens",
		metric.WithDescription("Tokens consumed, split by direction.")); err != nil {
		log.Error("create metric", "name", "ai_chat_tokens", "error", err)
	}
	if metrics.chatLatency, err = meter.Float64Histogram("ai_chat_latency_seconds",
		metric.WithDescription("End-to-end completion latency."),
		metric.WithExplicitBucketBoundaries(0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120)); err != nil {
		log.Error("create metric", "name", "ai_chat_latency_seconds", "error", err)
	}
	if metrics.chatCost, err = meter.Float64Counter("ai_chat_cost",
		metric.WithDescription("Configured cost of consumed tokens. Rates are operator policy, not an invoice.")); err != nil {
		log.Error("create metric", "name", "ai_chat_cost", "error", err)
	}
	if metrics.agentRuns, err = meter.Int64Counter("ai_agent_runs",
		metric.WithDescription("Agent runs, by assistant, outcome and whether the answer was grounded.")); err != nil {
		log.Error("create metric", "name", "ai_agent_runs", "error", err)
	}
	if metrics.agentSteps, err = meter.Int64Counter("ai_agent_steps",
		metric.WithDescription("Agent plan steps, by kind and outcome.")); err != nil {
		log.Error("create metric", "name", "ai_agent_steps", "error", err)
	}
	if metrics.retrievalScore, err = meter.Float64Histogram("ai_retrieval_best_score",
		metric.WithDescription("Best fused retrieval score per run. Reciprocal rank fusion scores are small by construction."),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.016, 0.02, 0.025, 0.033, 0.05)); err != nil {
		log.Error("create metric", "name", "ai_retrieval_best_score", "error", err)
	}
	if metrics.toolCalls, err = meter.Int64Counter("ai_tool_calls",
		metric.WithDescription("Tool executions, by tool and outcome including denied and throttled.")); err != nil {
		log.Error("create metric", "name", "ai_tool_calls", "error", err)
	}
	if metrics.toolLatency, err = meter.Float64Histogram("ai_tool_latency_seconds",
		metric.WithDescription("Tool execution latency."),
		metric.WithExplicitBucketBoundaries(0.01, 0.05, 0.1, 0.5, 1, 5, 15)); err != nil {
		log.Error("create metric", "name", "ai_tool_latency_seconds", "error", err)
	}
	if metrics.documents, err = meter.Int64Counter("ai_ingestion_documents",
		metric.WithDescription("Documents ingested, by outcome.")); err != nil {
		log.Error("create metric", "name", "ai_ingestion_documents", "error", err)
	}
	if metrics.chunks, err = meter.Int64Counter("ai_ingestion_chunks",
		metric.WithDescription("Chunks embedded and stored.")); err != nil {
		log.Error("create metric", "name", "ai_ingestion_chunks", "error", err)
	}
	if metrics.authFailures, err = meter.Int64Counter("ai_auth_failures",
		metric.WithDescription("Rejected credentials, by server-side reason.")); err != nil {
		log.Error("create metric", "name", "ai_auth_failures", "error", err)
	}
	if metrics.rateLimits, err = meter.Int64Counter("ai_rate_limit_denials",
		metric.WithDescription("Requests and tool calls refused by a quota.")); err != nil {
		log.Error("create metric", "name", "ai_rate_limit_denials", "error", err)
	}
	return metrics
}

// RecordCompletion records one model call.
//
// The label set deliberately excludes the API key and the company: a metric is
// scraped and retained by a shared Prometheus, and per-tenant labels would both
// explode cardinality and put tenancy in a store with weaker access controls
// than the audit tables that already hold it.
func (m *Metrics) RecordCompletion(ctx context.Context, logicalModel, provider, model, outcome string,
	promptTokens, completionTokens int, latency time.Duration) {
	if m == nil {
		return
	}
	base := []attribute.KeyValue{
		attribute.String("logical_model", logicalModel),
		attribute.String("provider", provider),
		attribute.String("model", model),
		attribute.String("outcome", outcome),
	}
	m.chatRequests.Add(ctx, 1, metric.WithAttributes(base...))
	m.chatLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(
		attribute.String("logical_model", logicalModel)))

	if promptTokens > 0 {
		m.chatTokens.Add(ctx, int64(promptTokens), metric.WithAttributes(
			append(base, attribute.String("direction", "prompt"))...))
	}
	if completionTokens > 0 {
		m.chatTokens.Add(ctx, int64(completionTokens), metric.WithAttributes(
			append(base, attribute.String("direction", "completion"))...))
	}

	if rate, priced := m.prices[logicalModel]; priced {
		total := float64(promptTokens+completionTokens) / 1000 * rate
		m.chatCost.Add(ctx, total, metric.WithAttributes(
			attribute.String("logical_model", logicalModel),
			attribute.String("currency", m.currency)))
	}
}

func (m *Metrics) RecordAgentRun(ctx context.Context, assistant, outcome string, grounded, conflicted bool, bestScore float64) {
	if m == nil {
		return
	}
	m.agentRuns.Add(ctx, 1, metric.WithAttributes(
		attribute.String("assistant", assistant),
		attribute.String("outcome", outcome),
		attribute.Bool("grounded", grounded),
		attribute.Bool("conflicted", conflicted)))
	// Zero means nothing was retrieved, which is worth seeing as its own bucket
	// rather than dropped.
	m.retrievalScore.Record(ctx, bestScore)
}

func (m *Metrics) RecordAgentStep(ctx context.Context, kind, outcome string) {
	if m == nil {
		return
	}
	m.agentSteps.Add(ctx, 1, metric.WithAttributes(
		attribute.String("kind", kind), attribute.String("outcome", outcome)))
}

func (m *Metrics) RecordToolCall(ctx context.Context, slug, outcome string, latency time.Duration) {
	if m == nil {
		return
	}
	m.toolCalls.Add(ctx, 1, metric.WithAttributes(
		attribute.String("tool", slug), attribute.String("outcome", outcome)))
	m.toolLatency.Record(ctx, latency.Seconds(), metric.WithAttributes(attribute.String("tool", slug)))
}

func (m *Metrics) RecordIngestion(ctx context.Context, outcome string, chunks int) {
	if m == nil {
		return
	}
	m.documents.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
	if chunks > 0 {
		m.chunks.Add(ctx, int64(chunks))
	}
}

// RecordAuthFailure takes the server-side reason, which is never sent to the
// client. It is bounded to the handful of reasons the store produces, so it
// cannot become a high-cardinality label.
func (m *Metrics) RecordAuthFailure(ctx context.Context, reason string) {
	if m == nil {
		return
	}
	m.authFailures.Add(ctx, 1, metric.WithAttributes(attribute.String("reason", reason)))
}

func (m *Metrics) RecordRateLimitDenial(ctx context.Context, kind string) {
	if m == nil {
		return
	}
	m.rateLimits.Add(ctx, 1, metric.WithAttributes(attribute.String("kind", kind)))
}
