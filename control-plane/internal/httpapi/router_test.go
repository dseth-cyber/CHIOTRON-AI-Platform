package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	"github.com/chiotron/ai-control-plane/internal/config"
)

func testConfig() config.Config {
	return config.Config{
		ServiceName:          "ai-control-plane",
		ServiceVersion:       "test",
		Environment:          "test",
		ComputeProvider:      "ollama",
		AllowedOrigins:       []string{"http://localhost:5173"},
		ReadinessTimeout:     time.Second,
		ComputeTimeout:       time.Second,
		ComputeHealthTimeout: time.Second,
	}
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func newTestRouter(checkers ...Checker) http.Handler {
	return NewRouter(Deps{
		Config:   testConfig(),
		Log:      quietLogger(),
		Metrics:  http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("# metrics")) }),
		Checkers: checkers,
	})
}

func get(t *testing.T, handler http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v (%s)", err, rec.Body.String())
	}
	return body
}

func okChecker(name string) Checker {
	return CheckerFunc{DependencyName: name, Probe: func(context.Context) error { return nil }}
}

func failingChecker(name string, reason string) Checker {
	return CheckerFunc{DependencyName: name, Probe: func(context.Context) error { return errors.New(reason) }}
}

// Liveness must not depend on backing services: restarting the process would
// not bring a failed database back.
func TestHealthzIgnoresDependencies(t *testing.T) {
	rec := get(t, newTestRouter(failingChecker("postgres", "down")), "/healthz", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 even with a failing dependency", rec.Code)
	}
	if status := decode(t, rec)["status"]; status != "ok" {
		t.Errorf("status field = %v, want ok", status)
	}
}

func TestReadyzReportsEveryDependency(t *testing.T) {
	rec := get(t, newTestRouter(okChecker("postgres"), okChecker("redis")), "/readyz", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decode(t, rec)
	if body["status"] != "ready" {
		t.Errorf("status field = %v, want ready", body["status"])
	}
	checks, ok := body["checks"].(map[string]any)
	if !ok || len(checks) != 2 {
		t.Fatalf("checks = %v, want one entry per dependency", body["checks"])
	}
}

func TestReadyzFailsWhenOneDependencyIsDown(t *testing.T) {
	rec := get(t, newTestRouter(okChecker("postgres"), failingChecker("redis", "connection refused")), "/readyz", nil)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	body := decode(t, rec)
	if body["status"] != "not-ready" {
		t.Errorf("status field = %v, want not-ready", body["status"])
	}

	checks := body["checks"].(map[string]any)
	redis := checks["redis"].(map[string]any)
	if redis["status"] != "unavailable" {
		t.Errorf("redis status = %v, want unavailable", redis["status"])
	}
	if redis["error"] != "connection refused" {
		t.Errorf("redis error = %v, want the underlying reason", redis["error"])
	}
	// A healthy dependency must still be reported so an operator can tell which
	// one actually failed.
	if postgres := checks["postgres"].(map[string]any); postgres["status"] != "ok" {
		t.Errorf("postgres status = %v, want ok", postgres["status"])
	}
}

func TestCORSOnlyReflectsAllowedOrigins(t *testing.T) {
	handler := newTestRouter(okChecker("postgres"))

	allowed := get(t, handler, "/healthz", map[string]string{"Origin": "http://localhost:5173"})
	if got := allowed.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("allow-origin = %q, want the request origin echoed", got)
	}

	denied := get(t, handler, "/healthz", map[string]string{"Origin": "http://evil.example"})
	if got := denied.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q for a disallowed origin, want empty", got)
	}
	// Vary must be set either way or a shared cache could serve one origin's
	// allow header to another.
	if got := denied.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin even when the origin is rejected", got)
	}
}

func TestPreflightShortCircuits(t *testing.T) {
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/platform", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("preflight response is missing Access-Control-Allow-Methods")
	}
}

func TestSecurityHeadersAlwaysPresent(t *testing.T) {
	rec := get(t, newTestRouter(), "/api/v1/platform", nil)

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	}
	for header, value := range want {
		if got := rec.Header().Get(header); got != value {
			t.Errorf("%s = %q, want %q", header, got, value)
		}
	}
}

func TestMetricsEndpointIsServed(t *testing.T) {
	rec := get(t, newTestRouter(), "/metrics", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "# metrics" {
		t.Errorf("body = %q, want the metrics handler output", rec.Body.String())
	}
}

// Loki logs are correlated with Tempo traces through the trace id
// (ARCHITECTURE-v1 section 9). That only works while the logging middleware
// runs inside the span otelhttp starts, so assert the ordering, not just the
// middleware in isolation.
func TestRequestLogCarriesTraceID(t *testing.T) {
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(tracesdk.NewTracerProvider(tracesdk.WithSampler(tracesdk.AlwaysSample())))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	var logged bytes.Buffer
	handler := NewRouter(Deps{
		Config: testConfig(),
		Log:    slog.New(slog.NewJSONHandler(&logged, nil)),
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/platform", nil))

	var entry map[string]any
	if err := json.Unmarshal(logged.Bytes(), &entry); err != nil {
		t.Fatalf("log line is not JSON: %v (%s)", err, logged.String())
	}
	traceID, ok := entry["traceId"].(string)
	if !ok || traceID == "" {
		t.Fatalf("request log has no traceId: %s", logged.String())
	}
	if entry["spanId"] == "" || entry["spanId"] == nil {
		t.Errorf("request log has no spanId: %s", logged.String())
	}
}

// Probes and scrapes would otherwise dominate the trace backend.
func TestOperationalEndpointsAreNotTraced(t *testing.T) {
	for _, path := range []string{"/healthz", "/readyz", "/metrics"} {
		if shouldTrace(httptest.NewRequest(http.MethodGet, path, nil)) {
			t.Errorf("shouldTrace(%q) = true, want false", path)
		}
	}
	if !shouldTrace(httptest.NewRequest(http.MethodGet, "/api/v1/platform", nil)) {
		t.Error("shouldTrace(/api/v1/platform) = false, want true")
	}
}

// One failing request must not take the Control Plane down.
func TestRecoverPanicReturns500(t *testing.T) {
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	handler := recoverPanic(quietLogger())(boom)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := decode(t, rec); body["error"] == nil {
		t.Error("response body has no error field")
	}
	// The panic value must not leak to the caller.
	if got := rec.Body.String(); got != "" && json.Valid([]byte(got)) && len(got) > 60 {
		t.Errorf("response body looks like it leaks internals: %q", got)
	}
}
