// Package config loads and validates Control Plane configuration from the
// environment. Model names, provider URLs and policy limits are configuration,
// not application constants (ARCHITECTURE-v1 section 4).
package config

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	ComputeProvider string
	OllamaBaseURL   string
	AllowedOrigins  []string
	LogLevel        slog.Level

	// Compute routing. Logical model ids are resolved to a provider and an
	// upstream model name at runtime (ARCHITECTURE-v1 section 4).
	ModelRoutes          string
	DefaultModel         string
	ComputeTimeout       time.Duration
	ComputeHealthTimeout time.Duration

	// DevUnauthenticatedChat exposes POST /api/v1/chat/completions with no
	// identity check. It exists only so the provider adapter can be exercised
	// end to end before the Gateway's JWT middleware lands, and must stay off
	// anywhere real users can reach the service.
	DevUnauthenticatedChat bool

	// Telemetry. VM4 emits OpenTelemetry to the existing Tempo/Loki stack and
	// is scraped by the existing Prometheus (ARCHITECTURE-v1 section 9); the
	// platform does not run a monitoring stack of its own.
	ServiceName      string
	ServiceVersion   string
	Environment      string
	OTLPEndpoint     string
	OTLPInsecure     bool
	TraceSampleRatio float64

	StartupTimeout   time.Duration
	ShutdownTimeout  time.Duration
	ReadinessTimeout time.Duration
}

// Load reads configuration from the process environment. It returns every
// validation problem at once so a misconfigured deployment fails fast with a
// complete report instead of one error per restart.
func Load(getenv func(string) string, version string) (Config, error) {
	cfg := Config{
		HTTPAddr:        stringVar(getenv, "HTTP_ADDR", ":8080"),
		DatabaseURL:     stringVar(getenv, "AI_DATABASE_URL", ""),
		RedisAddr:       stringVar(getenv, "REDIS_ADDR", ""),
		RedisPassword:   stringVar(getenv, "REDIS_PASSWORD", ""),
		ComputeProvider: stringVar(getenv, "COMPUTE_PROVIDER", "ollama"),
		OllamaBaseURL:   stringVar(getenv, "OLLAMA_BASE_URL", "http://ollama:11434"),
		AllowedOrigins:  listVar(getenv, "CORS_ALLOWED_ORIGINS", "http://localhost:5173"),
		ModelRoutes:     stringVar(getenv, "MODEL_ROUTES", "default=ollama/qwen2.5:0.5b"),
		DefaultModel:    stringVar(getenv, "DEFAULT_MODEL", "default"),
		ServiceName:     stringVar(getenv, "OTEL_SERVICE_NAME", "ai-control-plane"),
		ServiceVersion:  version,
		Environment:     stringVar(getenv, "DEPLOYMENT_ENVIRONMENT", "development"),
		OTLPEndpoint:    stringVar(getenv, "OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}

	var problems []string
	fail := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }

	if cfg.DatabaseURL == "" {
		fail("AI_DATABASE_URL is required")
	}
	if cfg.RedisAddr == "" {
		fail("REDIS_ADDR is required")
	}
	if len(cfg.AllowedOrigins) == 0 {
		fail("CORS_ALLOWED_ORIGINS must list at least one origin")
	}
	switch cfg.ComputeProvider {
	case "ollama", "vllm", "nim", "external":
	default:
		fail("COMPUTE_PROVIDER %q is not a known provider (ollama, vllm, nim, external)", cfg.ComputeProvider)
	}

	var err error
	if cfg.RedisDB, err = intVar(getenv, "REDIS_DB", 0); err != nil {
		fail("%v", err)
	}
	if cfg.LogLevel, err = levelVar(getenv, "LOG_LEVEL", slog.LevelInfo); err != nil {
		fail("%v", err)
	}
	if cfg.OTLPInsecure, err = boolVar(getenv, "OTEL_EXPORTER_OTLP_INSECURE", true); err != nil {
		fail("%v", err)
	}
	if cfg.TraceSampleRatio, err = ratioVar(getenv, "OTEL_TRACES_SAMPLER_ARG", 1.0); err != nil {
		fail("%v", err)
	}
	if cfg.StartupTimeout, err = durationVar(getenv, "STARTUP_TIMEOUT", 30*time.Second); err != nil {
		fail("%v", err)
	}
	if cfg.ShutdownTimeout, err = durationVar(getenv, "SHUTDOWN_TIMEOUT", 15*time.Second); err != nil {
		fail("%v", err)
	}
	if cfg.ReadinessTimeout, err = durationVar(getenv, "READINESS_TIMEOUT", 2*time.Second); err != nil {
		fail("%v", err)
	}
	if cfg.ComputeTimeout, err = durationVar(getenv, "COMPUTE_TIMEOUT", 120*time.Second); err != nil {
		fail("%v", err)
	}
	if cfg.ComputeHealthTimeout, err = durationVar(getenv, "COMPUTE_HEALTH_TIMEOUT", 5*time.Second); err != nil {
		fail("%v", err)
	}
	if cfg.DevUnauthenticatedChat, err = boolVar(getenv, "DEV_UNAUTHENTICATED_CHAT", false); err != nil {
		fail("%v", err)
	}

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return cfg, nil
}

// TracingEnabled reports whether traces have somewhere to go. Without a
// collector endpoint the service still runs and still serves metrics; it just
// does not export spans.
func (c Config) TracingEnabled() bool { return c.OTLPEndpoint != "" }

func stringVar(getenv func(string) string, key, fallback string) string {
	if value := strings.TrimSpace(getenv(key)); value != "" {
		return value
	}
	return fallback
}

func listVar(getenv func(string) string, key, fallback string) []string {
	var values []string
	for _, part := range strings.Split(stringVar(getenv, key, fallback), ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func intVar(getenv func(string) string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return value, nil
}

func boolVar(getenv func(string) string, key string, fallback bool) (bool, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false, got %q", key, raw)
	}
	return value, nil
}

func ratioVar(getenv func(string) string, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number, got %q", key, raw)
	}
	if value < 0 || value > 1 {
		return 0, fmt.Errorf("%s must be between 0 and 1, got %q", key, raw)
	}
	return value, nil
}

func durationVar(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 30s, got %q", key, raw)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero, got %q", key, raw)
	}
	return value, nil
}

func levelVar(getenv func(string) string, key string, fallback slog.Level) (slog.Level, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("%s must be debug, info, warn or error, got %q", key, raw)
	}
	return level, nil
}

// Redacted returns log-safe attributes. Connection strings carry credentials
// and must never reach structured logs (ARCHITECTURE-v1 section 5).
func (c Config) Redacted() []any {
	return []any{
		"httpAddr", c.HTTPAddr,
		"serviceName", c.ServiceName,
		"serviceVersion", c.ServiceVersion,
		"environment", c.Environment,
		"computeProvider", c.ComputeProvider,
		"ollamaBaseURL", c.OllamaBaseURL,
		"modelRoutes", c.ModelRoutes,
		"defaultModel", c.DefaultModel,
		"devUnauthenticatedChat", c.DevUnauthenticatedChat,
		"redisAddr", c.RedisAddr,
		"redisDB", c.RedisDB,
		"allowedOrigins", c.AllowedOrigins,
		"logLevel", c.LogLevel.String(),
		"tracingEnabled", c.TracingEnabled(),
		"otlpEndpoint", c.OTLPEndpoint,
		"traceSampleRatio", c.TraceSampleRatio,
	}
}
