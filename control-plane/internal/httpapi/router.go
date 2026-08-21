// Package httpapi exposes the Control Plane's HTTP surface. Today that is
// liveness, readiness, metrics and platform discovery; the Gateway endpoints in
// ARCHITECTURE-v1 section 7 are layered on top of this router.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/config"
	"github.com/chiotron/ai-control-plane/internal/portal"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/telemetry"
)

// Checker reports whether one backing dependency is usable right now.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// CheckerFunc adapts a probe function into a Checker.
type CheckerFunc struct {
	DependencyName string
	Probe          func(ctx context.Context) error
}

func (c CheckerFunc) Name() string                    { return c.DependencyName }
func (c CheckerFunc) Check(ctx context.Context) error { return c.Probe(ctx) }

// ComputeRegistry is the routing surface the handlers use.
//
// It is an interface rather than *provider.Registry because the live registry
// can be rebuilt while the process serves: an operator changing a route from
// the Admin UI must not need a restart (ARCHITECTURE-v1 section 46).
type ComputeRegistry interface {
	Resolve(logical string) (provider.LLM, provider.Route, error)
	Chat(ctx context.Context, logical string, req provider.ChatRequest) (provider.ChatResponse, provider.Route, error)
	ChatStream(ctx context.Context, logical string, req provider.ChatRequest,
		emit func(provider.Chunk) error) (provider.ChatResponse, provider.Route, error)
	DefaultModel() string
	Routes() []provider.Route
	Providers() []provider.LLM
}

// Deps is everything the router needs from the rest of the process.
type Deps struct {
	Config        config.Config
	Log           *slog.Logger
	Metrics       http.Handler
	Checkers      []Checker
	Compute       ComputeRegistry
	Auth          Authenticator
	Keys          KeyAdmin
	Limiter       RateLimiter
	Audit         AuditRecorder
	Assistants    AssistantCatalogue
	Conversations ConversationStore
	Knowledge     Knowledge
	Agent         Agent
	Favorites     FavoriteStore
	Settings      SettingsAdmin
	Prompts       PromptAdmin
	// Providers is the model-routing registry. ReloadCompute applies a change to
	// the running process, and CredentialStorage reports whether a provider
	// credential can be stored at all, so the UI can say why not.
	Providers         ProviderAdmin
	ReloadCompute     Reloader
	CredentialStorage bool
	// Instruments records platform metrics. Metrics above is the scrape handler
	// that exposes them. Both are optional: losing a metric must not stop the
	// platform serving requests.
	Instruments *telemetry.Metrics
}

// authenticated reports whether the authenticated routes can be served. They
// are registered together or not at all, so a partially wired process cannot
// expose an endpoint with its guard missing.
func (d Deps) authenticated() bool {
	return d.Auth != nil && d.Limiter != nil && d.Audit != nil
}

// contextWithTimeout bounds a handler's downstream work without outliving the
// request itself.
func contextWithTimeout(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}

type checkResult struct {
	Status    string `json:"status"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

// NewRouter builds the Control Plane handler with its middleware applied.
func NewRouter(d Deps) http.Handler {
	cfg := d.Config
	mux := http.NewServeMux()

	// Liveness answers without touching dependencies: a failing database means
	// not ready, not dead, and restarting the process would not fix it.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"service":         cfg.ServiceName,
			"version":         cfg.ServiceVersion,
			"computeProvider": cfg.ComputeProvider,
			"time":            time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), cfg.ReadinessTimeout)
		defer cancel()

		results := make(map[string]checkResult, len(d.Checkers))
		ready := true
		for _, checker := range d.Checkers {
			started := time.Now()
			result := checkResult{Status: "ok"}
			if err := checker.Check(ctx); err != nil {
				result.Status = "unavailable"
				result.Error = err.Error()
				ready = false
			}
			result.LatencyMs = time.Since(started).Milliseconds()
			results[checker.Name()] = result
		}

		status, label := http.StatusOK, "ready"
		if !ready {
			status, label = http.StatusServiceUnavailable, "not-ready"
		}
		writeJSON(w, status, map[string]any{
			"status": label,
			"checks": results,
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	if d.Metrics != nil {
		mux.Handle("GET /metrics", d.Metrics)
	}

	mux.HandleFunc("GET /api/v1/platform", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":            "CHIOTRON Enterprise AI Platform",
			"plane":           "control",
			"version":         cfg.ServiceVersion,
			"environment":     cfg.Environment,
			"capabilities":    []string{"gateway", "orchestration", "audit", "model-provider-routing"},
			"computeProvider": cfg.ComputeProvider,
		})
	})

	if d.authenticated() {
		// Lets a client discover what its own credential may do, so a UI can
		// hide what the caller cannot use instead of probing for 403s. UI
		// filtering stays convenience only: the backend still authorizes every
		// action (ARCHITECTURE-v1 section 5).
		mux.HandleFunc("GET /api/v1/me", d.guard(anyScope, func(w http.ResponseWriter, r *http.Request) {
			identity, _ := auth.IdentityFrom(r.Context())
			writeJSON(w, http.StatusOK, identity)
		}))
	}

	registerCompute(mux, d)
	registerAssistants(mux, d)
	registerConversations(mux, d)
	registerChat(mux, d)
	registerKnowledge(mux, d)
	registerAgent(mux, d)
	registerFavorites(mux, d)
	registerProviders(mux, d)
	registerAdmin(mux, d)
	registerSettings(mux, d)
	registerPrompts(mux, d)

	// Single Go Binary: Serve embedded Portal Web UI for all non-API routes
	mux.Handle("/", portal.Handler())

	// Ordering matters, outermost first:
	//   recoverPanic  - nothing below it can take the process down
	//   otelhttp      - starts the span and records server metrics
	//   requestLog    - runs inside the span, so it can log its trace id
	//   securityHeaders / cors - response shaping, closest to the routes
	handler := chain(mux,
		requestLog(d.Log),
		securityHeaders,
		cors(cfg.AllowedOrigins),
	)
	handler = otelhttp.NewHandler(handler, cfg.ServiceName,
		otelhttp.WithFilter(shouldTrace),
		otelhttp.WithSpanNameFormatter(spanName),
	)
	return recoverPanic(d.Log)(handler)
}

// shouldTrace keeps operational polling out of the trace backend. Kubernetes
// probes and Prometheus scrape continuously and would otherwise dominate Tempo.
func shouldTrace(r *http.Request) bool {
	switch r.URL.Path {
	case "/healthz", "/readyz", "/metrics":
		return false
	}
	return true
}

// spanName keeps span names low-cardinality. Routes with path parameters must
// be named from the matched pattern rather than the raw path when they arrive.
func spanName(_ string, r *http.Request) string {
	return r.Method + " " + r.URL.Path
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
