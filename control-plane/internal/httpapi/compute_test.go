package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

type fakeProvider struct {
	name      string
	healthErr error
	models    []provider.Model
	chatErr   error
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Health(context.Context) error { return f.healthErr }

func (f *fakeProvider) Models(context.Context) ([]provider.Model, error) {
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	return f.models, nil
}

func (f *fakeProvider) Chat(_ context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	if f.chatErr != nil {
		return provider.ChatResponse{}, f.chatErr
	}
	return provider.ChatResponse{
		Model: req.Model, Content: "hello", FinishReason: "stop",
		Usage:     provider.Usage{PromptTokens: 11, CompletionTokens: 4, TotalTokens: 15},
		LatencyMs: 42,
	}, nil
}

type computeFixture struct {
	handler http.Handler
	audit   *fakeAudit
	limiter *fakeLimiter
}

func newComputeFixture(t *testing.T, p provider.LLM, mutate ...func(*Deps)) computeFixture {
	t.Helper()
	routes, err := provider.ParseRoutes("default=ollama/qwen2.5:0.5b")
	if err != nil {
		t.Fatalf("ParseRoutes() returned error: %v", err)
	}
	registry, err := provider.NewRegistry(routes, "default", p)
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}

	recorder := &fakeAudit{}
	limiter := allowingLimiter()
	deps := Deps{
		Config:  testConfig(),
		Log:     quietLogger(),
		Compute: registry,
		Auth:    &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter: limiter,
		Audit:   recorder,
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return computeFixture{handler: NewRouter(deps), audit: recorder, limiter: limiter}
}

func availableProvider() *fakeProvider {
	return &fakeProvider{name: "ollama", models: []provider.Model{{ID: "qwen2.5:0.5b", Family: "qwen2"}}}
}

// Losing the compute plane degrades model calls only. The endpoint reports on
// VM5; it does not fail with it.
func TestComputeHealthReportsUnavailableWithoutFailing(t *testing.T) {
	down := &fakeProvider{name: "ollama", healthErr: provider.ErrUnavailable}
	rec := authedGet(t, newComputeFixture(t, down).handler, "/api/v1/compute/health")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 - this endpoint reports, it does not fail", rec.Code)
	}
	body := decode(t, rec)
	if body["status"] != "unavailable" {
		t.Errorf("status field = %v, want unavailable", body["status"])
	}
	providers := body["providers"].(map[string]any)
	if providers["ollama"].(map[string]any)["error"] == nil {
		t.Error("provider entry has no error detail")
	}
}

func TestComputeHealthReportsAvailable(t *testing.T) {
	rec := authedGet(t, newComputeFixture(t, availableProvider()).handler, "/api/v1/compute/health")

	body := decode(t, rec)
	if body["status"] != "available" {
		t.Fatalf("status field = %v, want available", body["status"])
	}
	ollama := body["providers"].(map[string]any)["ollama"].(map[string]any)
	if models, ok := ollama["models"].([]any); !ok || len(models) != 1 {
		t.Errorf("provider entry does not list loaded models: %v", ollama)
	}
}

// A failing compute plane must not make the Control Plane itself not-ready.
func TestComputeFailureDoesNotAffectReadiness(t *testing.T) {
	down := &fakeProvider{name: "ollama", healthErr: provider.ErrUnavailable}
	rec := get(t, newComputeFixture(t, down).handler, "/readyz", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz status = %d, want 200 when only the compute plane is down", rec.Code)
	}
}

func TestModelsReportsRouteAvailability(t *testing.T) {
	rec := authedGet(t, newComputeFixture(t, availableProvider()).handler, "/api/v1/models")

	body := decode(t, rec)
	if body["default"] != "default" {
		t.Errorf("default = %v, want default", body["default"])
	}
	models := body["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("models = %v, want one entry per route", models)
	}
	entry := models[0].(map[string]any)
	if entry["logical"] != "default" || entry["model"] != "qwen2.5:0.5b" || entry["provider"] != "ollama" {
		t.Errorf("model entry = %v, want the resolved route", entry)
	}
	if entry["available"] != true {
		t.Errorf("available = %v, want true when the provider has the model loaded", entry["available"])
	}
}

func TestModelsMarksMissingUpstreamModelUnavailable(t *testing.T) {
	rec := authedGet(t, newComputeFixture(t, &fakeProvider{name: "ollama"}).handler, "/api/v1/models")

	entry := decode(t, rec)["models"].([]any)[0].(map[string]any)
	if entry["available"] != false {
		t.Errorf("available = %v, want false when the route's model is not loaded", entry["available"])
	}
}

func TestChatReturnsCompletion(t *testing.T) {
	fixture := newComputeFixture(t, availableProvider())
	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["content"] != "hello" {
		t.Errorf("content = %v, want hello", body["content"])
	}
	// The caller asked for no model, so the default route must be reported back.
	if body["logicalModel"] != "default" || body["provider"] != "ollama" {
		t.Errorf("response = %v, want the resolved route echoed", body)
	}
}

// Every model call creates usage metadata (ARCHITECTURE-v1 section 5).
func TestChatRecordsUsage(t *testing.T) {
	fixture := newComputeFixture(t, availableProvider())
	authedPost(t, fixture.handler, "/api/v1/chat/completions", `{"messages":[{"role":"user","content":"hi"}]}`)

	if len(fixture.audit.usage) != 1 {
		t.Fatalf("recorded %d usage events, want 1", len(fixture.audit.usage))
	}
	usage := fixture.audit.usage[0]
	if usage.TotalTokens != 15 || usage.PromptTokens != 11 || usage.CompletionTokens != 4 {
		t.Errorf("usage tokens = %+v, want the provider's counts", usage)
	}
	if usage.Outcome != audit.OutcomeSuccess {
		t.Errorf("outcome = %q, want success", usage.Outcome)
	}
	if usage.APIKeyID != fullyScopedIdentity().KeyID {
		t.Errorf("apiKeyId = %q, want the calling key", usage.APIKeyID)
	}
}

// A failed call still consumed capacity, so it must still be accounted for.
func TestChatRecordsUsageOnFailure(t *testing.T) {
	down := &fakeProvider{name: "ollama", chatErr: provider.ErrUnavailable}
	fixture := newComputeFixture(t, down)
	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions", `{"messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if len(fixture.audit.usage) != 1 {
		t.Fatalf("recorded %d usage events, want 1 even though the call failed", len(fixture.audit.usage))
	}
	if outcome := fixture.audit.usage[0].Outcome; outcome != audit.OutcomeFailure {
		t.Errorf("outcome = %q, want failure", outcome)
	}
}

func TestChatRejectsBadRequests(t *testing.T) {
	handler := newComputeFixture(t, availableProvider()).handler

	cases := map[string]struct {
		body string
		want int
	}{
		"not json":      {`{`, http.StatusBadRequest},
		"no messages":   {`{"messages":[]}`, http.StatusBadRequest},
		"unknown field": {`{"messages":[{"role":"user","content":"hi"}],"topP":0.9}`, http.StatusBadRequest},
		"unknown model": {`{"model":"nope","messages":[{"role":"user","content":"hi"}]}`, http.StatusNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := authedPost(t, handler, "/api/v1/chat/completions", tc.body); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// Compute routes must never be reachable without a credential.
func TestComputeRoutesRequireAuthentication(t *testing.T) {
	handler := newComputeFixture(t, availableProvider()).handler

	for _, path := range []string{"/api/v1/models", "/api/v1/compute/health"} {
		if rec := get(t, handler, path, nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("GET %s without a key = %d, want 401", path, rec.Code)
		}
	}
}

// A key without chat:completions may still read the catalogue, and vice versa.
func TestComputeRoutesEnforceScopes(t *testing.T) {
	readOnly := auth.Identity{KeyID: "k", Scopes: []string{auth.ScopeModelsRead}, RateLimitPerMinute: 60}
	fixture := newComputeFixture(t, availableProvider(), func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: readOnly}
	})

	if rec := authedGet(t, fixture.handler, "/api/v1/models"); rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/models with models:read = %d, want 200", rec.Code)
	}

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions", `{"messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("chat without chat:completions = %d, want 403", rec.Code)
	}
	if event := fixture.audit.lastEvent(t); event.Outcome != audit.OutcomeDenied {
		t.Errorf("denied request recorded outcome %q, want denied", event.Outcome)
	}
}

func TestChatSurfacesUnexpectedProviderErrorAs502(t *testing.T) {
	broken := &fakeProvider{name: "ollama", chatErr: errors.New("boom")}
	rec := authedPost(t, newComputeFixture(t, broken).handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}
