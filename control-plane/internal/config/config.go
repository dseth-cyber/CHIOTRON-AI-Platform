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

	// DefaultRateLimitPerMinute is the quota applied to a new API key when the
	// creating call does not name one. Per-key limits live on the key record.
	DefaultRateLimitPerMinute int

	// Conversation retention and prompt logging are policy settings
	// (ARCHITECTURE-v1 section 5). With PersistPrompts off, a turn is still
	// recorded so history keeps its shape, without the message text.
	PersistPrompts   bool
	HistoryTurnLimit int

	// Knowledge platform. The storage provider of record and the classification
	// policy are open decisions (ARCHITECTURE-v1 section 13 items 5 and 6), so
	// both are configuration and the code names neither.
	StorageRoot          string
	EmbeddingModel       string
	EmbeddingDimensions  int
	ClassificationLevels []string
	ChunkSize            int
	ChunkOverlap         int
	MaxDocumentBytes     int
	IngestionInterval    time.Duration
	IngestionBatch       int

	// Agent planner policy. Retrieval depth and the score a round must beat are
	// tuning, not business logic, so both are configuration.
	AgentMaxSteps       int
	AgentTopK           int
	AgentMinScore       float64
	AgentConflictMargin float64

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
		StorageRoot:     stringVar(getenv, "STORAGE_ROOT", "/var/lib/chiotron/documents"),
		EmbeddingModel:  stringVar(getenv, "EMBEDDING_MODEL", "nomic-embed-text"),
		ClassificationLevels: listVar(getenv, "CLASSIFICATION_LEVELS",
			"public,internal,confidential,restricted"),
		ServiceName:    stringVar(getenv, "OTEL_SERVICE_NAME", "ai-control-plane"),
		ServiceVersion: version,
		Environment:    stringVar(getenv, "DEPLOYMENT_ENVIRONMENT", "development"),
		OTLPEndpoint:   stringVar(getenv, "OTEL_EXPORTER_OTLP_ENDPOINT", ""),
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
	if cfg.DefaultRateLimitPerMinute, err = intVar(getenv, "DEFAULT_RATE_LIMIT_PER_MINUTE", 60); err != nil {
		fail("%v", err)
	} else if cfg.DefaultRateLimitPerMinute <= 0 {
		fail("DEFAULT_RATE_LIMIT_PER_MINUTE must be greater than zero, got %d", cfg.DefaultRateLimitPerMinute)
	}
	if cfg.PersistPrompts, err = boolVar(getenv, "PERSIST_PROMPTS", true); err != nil {
		fail("%v", err)
	}
	if cfg.HistoryTurnLimit, err = intVar(getenv, "HISTORY_TURN_LIMIT", 20); err != nil {
		fail("%v", err)
	} else if cfg.HistoryTurnLimit <= 0 {
		fail("HISTORY_TURN_LIMIT must be greater than zero, got %d", cfg.HistoryTurnLimit)
	}

	if len(cfg.ClassificationLevels) == 0 {
		fail("CLASSIFICATION_LEVELS must list at least one level, least sensitive first")
	}
	// The schema pins the vector width, so a mismatch has to be caught here
	// rather than after a re-embedding pass has written unusable rows.
	if cfg.EmbeddingDimensions, err = intVar(getenv, "EMBEDDING_DIMENSIONS", 768); err != nil {
		fail("%v", err)
	} else if cfg.EmbeddingDimensions != 768 {
		fail("EMBEDDING_DIMENSIONS is %d but the chunks table declares vector(768); "+
			"changing the embedding model needs a migration and a re-embedding pass",
			cfg.EmbeddingDimensions)
	}
	if cfg.ChunkSize, err = intVar(getenv, "CHUNK_SIZE", 1200); err != nil {
		fail("%v", err)
	}
	if cfg.ChunkOverlap, err = intVar(getenv, "CHUNK_OVERLAP", 150); err != nil {
		fail("%v", err)
	}
	if cfg.MaxDocumentBytes, err = intVar(getenv, "MAX_DOCUMENT_BYTES", 5<<20); err != nil {
		fail("%v", err)
	} else if cfg.MaxDocumentBytes <= 0 {
		fail("MAX_DOCUMENT_BYTES must be greater than zero, got %d", cfg.MaxDocumentBytes)
	}
	if cfg.IngestionInterval, err = durationVar(getenv, "INGESTION_INTERVAL", 5*time.Second); err != nil {
		fail("%v", err)
	}
	if cfg.IngestionBatch, err = intVar(getenv, "INGESTION_BATCH", 4); err != nil {
		fail("%v", err)
	} else if cfg.IngestionBatch <= 0 {
		fail("INGESTION_BATCH must be greater than zero, got %d", cfg.IngestionBatch)
	}

	if cfg.AgentMaxSteps, err = intVar(getenv, "AGENT_MAX_STEPS", 3); err != nil {
		fail("%v", err)
	} else if cfg.AgentMaxSteps < 1 {
		fail("AGENT_MAX_STEPS must be at least 1, got %d", cfg.AgentMaxSteps)
	}
	if cfg.AgentTopK, err = intVar(getenv, "AGENT_TOP_K", 5); err != nil {
		fail("%v", err)
	} else if cfg.AgentTopK < 1 {
		fail("AGENT_TOP_K must be at least 1, got %d", cfg.AgentTopK)
	}
	// Reciprocal rank fusion scores are small by construction: a single top-ranked
	// hit contributes 1/(60+1). The default is deliberately just under that, so a
	// lone strong match counts as evidence and nothing else does.
	if cfg.AgentMinScore, err = floatVar(getenv, "AGENT_MIN_SCORE", 0.016); err != nil {
		fail("%v", err)
	}
	if cfg.AgentConflictMargin, err = ratioVar(getenv, "AGENT_CONFLICT_MARGIN", 0.15); err != nil {
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

// floatVar reads a non-negative number. Unlike ratioVar it is not capped at 1,
// because a fused relevance score has no natural upper bound.
func floatVar(getenv func(string) string, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number, got %q", key, raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s cannot be negative, got %q", key, raw)
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
		"defaultRateLimitPerMinute", c.DefaultRateLimitPerMinute,
		"persistPrompts", c.PersistPrompts,
		"historyTurnLimit", c.HistoryTurnLimit,
		"storageRoot", c.StorageRoot,
		"embeddingModel", c.EmbeddingModel,
		"embeddingDimensions", c.EmbeddingDimensions,
		"classificationLevels", c.ClassificationLevels,
		"chunkSize", c.ChunkSize,
		"chunkOverlap", c.ChunkOverlap,
		"redisAddr", c.RedisAddr,
		"redisDB", c.RedisDB,
		"allowedOrigins", c.AllowedOrigins,
		"logLevel", c.LogLevel.String(),
		"tracingEnabled", c.TracingEnabled(),
		"otlpEndpoint", c.OTLPEndpoint,
		"traceSampleRatio", c.TraceSampleRatio,
	}
}
