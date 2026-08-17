package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	return provider.ChatResponse{Model: req.Model, Content: "hello", FinishReason: "stop"}, nil
}

func computeRouter(t *testing.T, devChat bool, p *fakeProvider) http.Handler {
	t.Helper()
	routes, err := provider.ParseRoutes("default=ollama/qwen2.5:0.5b")
	if err != nil {
		t.Fatalf("ParseRoutes() returned error: %v", err)
	}
	registry, err := provider.NewRegistry(routes, "default", p)
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}

	cfg := testConfig()
	cfg.DevUnauthenticatedChat = devChat
	return NewRouter(Deps{Config: cfg, Log: quietLogger(), Compute: registry})
}

func availableProvider() *fakeProvider {
	return &fakeProvider{name: "ollama", models: []provider.Model{{ID: "qwen2.5:0.5b", Family: "qwen2"}}}
}

// Losing the compute plane degrades model calls only. The endpoint reports on
// VM5; it does not fail with it.
func TestComputeHealthReportsUnavailableWithoutFailing(t *testing.T) {
	down := &fakeProvider{name: "ollama", healthErr: provider.ErrUnavailable}
	rec := get(t, computeRouter(t, false, down), "/api/v1/compute/health", nil)

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
	rec := get(t, computeRouter(t, false, availableProvider()), "/api/v1/compute/health", nil)

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
	rec := get(t, computeRouter(t, false, down), "/readyz", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz status = %d, want 200 when only the compute plane is down", rec.Code)
	}
}

func TestModelsReportsRouteAvailability(t *testing.T) {
	rec := get(t, computeRouter(t, false, availableProvider()), "/api/v1/models", nil)

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
	empty := &fakeProvider{name: "ollama"}
	rec := get(t, computeRouter(t, false, empty), "/api/v1/models", nil)

	entry := decode(t, rec)["models"].([]any)[0].(map[string]any)
	if entry["available"] != false {
		t.Errorf("available = %v, want false when the route's model is not loaded", entry["available"])
	}
}

// The unauthenticated chat route is a development affordance and must be
// absent unless it is explicitly switched on.
func TestChatRouteAbsentByDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions",
		strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	rec := httptest.NewRecorder()
	computeRouter(t, false, availableProvider()).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 while DEV_UNAUTHENTICATED_CHAT is off", rec.Code)
	}
}

func postChat(t *testing.T, handler http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestChatReturnsCompletion(t *testing.T) {
	rec := postChat(t, computeRouter(t, true, availableProvider()),
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

func TestChatRejectsBadRequests(t *testing.T) {
	handler := computeRouter(t, true, availableProvider())

	cases := map[string]struct {
		body string
		want int
	}{
		"not json":      {`{`, http.StatusBadRequest},
		"no messages":   {`{"messages":[]}`, http.StatusBadRequest},
		"unknown field": {`{"messages":[{"role":"user","content":"hi"}],"stream":true}`, http.StatusBadRequest},
		"unknown model": {`{"model":"nope","messages":[{"role":"user","content":"hi"}]}`, http.StatusNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := postChat(t, handler, tc.body); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// An unreachable compute plane is a 503 about VM5, not a 500 about VM4.
func TestChatReportsComputeOutageAs503(t *testing.T) {
	down := &fakeProvider{name: "ollama", chatErr: provider.ErrUnavailable}
	rec := postChat(t, computeRouter(t, true, down), `{"messages":[{"role":"user","content":"hi"}]}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}
