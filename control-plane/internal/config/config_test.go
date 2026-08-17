package config

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

// minimal returns an environment with only the required variables set.
func minimal() map[string]string {
	return map[string]string{
		"AI_DATABASE_URL": "postgres://user:secret@postgres:5432/chiotron_ai?sslmode=disable",
		"REDIS_ADDR":      "redis:6379",
	}
}

func lookup(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(lookup(minimal()), "1.2.3")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.ComputeProvider != "ollama" {
		t.Errorf("ComputeProvider = %q, want ollama", cfg.ComputeProvider)
	}
	if cfg.ServiceVersion != "1.2.3" {
		t.Errorf("ServiceVersion = %q, want 1.2.3", cfg.ServiceVersion)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.ReadinessTimeout != 2*time.Second {
		t.Errorf("ReadinessTimeout = %v, want 2s", cfg.ReadinessTimeout)
	}
	if want := []string{"http://localhost:5173"}; len(cfg.AllowedOrigins) != 1 || cfg.AllowedOrigins[0] != want[0] {
		t.Errorf("AllowedOrigins = %v, want %v", cfg.AllowedOrigins, want)
	}
	if cfg.TracingEnabled() {
		t.Error("TracingEnabled() = true with no OTLP endpoint, want false")
	}
}

func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	_, err := Load(lookup(map[string]string{"COMPUTE_PROVIDER": "banana"}), "dev")
	if err == nil {
		t.Fatal("Load() succeeded on an empty environment, want error")
	}

	for _, want := range []string{"AI_DATABASE_URL", "REDIS_ADDR", "banana"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q; every problem should be reported at once", err, want)
		}
	}
}

func TestLoadRejectsMalformedValues(t *testing.T) {
	cases := map[string]struct{ key, value string }{
		"log level":     {"LOG_LEVEL", "chatty"},
		"duration":      {"SHUTDOWN_TIMEOUT", "soon"},
		"zero duration": {"SHUTDOWN_TIMEOUT", "0s"},
		"integer":       {"REDIS_DB", "second"},
		"sample ratio":  {"OTEL_TRACES_SAMPLER_ARG", "1.5"},
		"boolean":       {"OTEL_EXPORTER_OTLP_INSECURE", "maybe"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env := minimal()
			env[tc.key] = tc.value
			if _, err := Load(lookup(env), "dev"); err == nil {
				t.Fatalf("Load() accepted %s=%q, want error", tc.key, tc.value)
			}
		})
	}
}

func TestLoadParsesOverrides(t *testing.T) {
	env := minimal()
	env["CORS_ALLOWED_ORIGINS"] = "https://ai.example.com, https://portal.example.com ,"
	env["OTEL_EXPORTER_OTLP_ENDPOINT"] = "http://collector:4318"
	env["OTEL_TRACES_SAMPLER_ARG"] = "0.25"
	env["LOG_LEVEL"] = "debug"

	cfg, err := Load(lookup(env), "dev")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if len(cfg.AllowedOrigins) != 2 {
		t.Errorf("AllowedOrigins = %v, want 2 entries with blanks dropped", cfg.AllowedOrigins)
	}
	if !cfg.TracingEnabled() {
		t.Error("TracingEnabled() = false with an OTLP endpoint set, want true")
	}
	if cfg.TraceSampleRatio != 0.25 {
		t.Errorf("TraceSampleRatio = %v, want 0.25", cfg.TraceSampleRatio)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
	}
}

// Connection strings carry credentials and must never reach structured logs
// (ARCHITECTURE-v1 section 5).
func TestRedactedOmitsCredentials(t *testing.T) {
	env := minimal()
	env["REDIS_PASSWORD"] = "redis-secret"

	cfg, err := Load(lookup(env), "dev")
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	rendered := strings.ToLower(fmt.Sprint(cfg.Redacted()...))
	for _, secret := range []string{"secret", "redis-secret", cfg.DatabaseURL} {
		if strings.Contains(rendered, strings.ToLower(secret)) {
			t.Errorf("Redacted() leaked %q in %q", secret, rendered)
		}
	}
}
