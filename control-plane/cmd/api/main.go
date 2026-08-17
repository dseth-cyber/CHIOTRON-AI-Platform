// Command api runs the CHIOTRON AI Control Plane.
//
// It owns configuration, its own database schema and the HTTP surface the
// portal and future Gateway build on. It never talks to a model provider on
// behalf of a browser: users reach compute only through this plane
// (ARCHITECTURE-v1 sections 1 and 5).
//
// Usage:
//
//	control-plane                serve
//	control-plane apikey create  mint an API key and print it once
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/chiotron/ai-control-plane/internal/agent"
	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/config"
	"github.com/chiotron/ai-control-plane/internal/conversation"
	"github.com/chiotron/ai-control-plane/internal/httpapi"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
	"github.com/chiotron/ai-control-plane/internal/migrate"
	"github.com/chiotron/ai-control-plane/internal/migrations"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/provider/ollama"
	"github.com/chiotron/ai-control-plane/internal/ratelimit"
	"github.com/chiotron/ai-control-plane/internal/storage"
	"github.com/chiotron/ai-control-plane/internal/store"
	"github.com/chiotron/ai-control-plane/internal/telemetry"
	"github.com/chiotron/ai-control-plane/internal/tool"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var err error
	if len(os.Args) > 1 && os.Args[1] == "apikey" {
		err = runAPIKey(os.Args[2:])
	} else {
		err = run()
	}
	if err != nil {
		slog.Error("control plane stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv, version)
	if err != nil {
		return err
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(log)
	log.Info("starting ai-control-plane", cfg.Redacted()...)

	// Cancel startup and in-flight work as soon as the platform asks us to stop.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tel, err := telemetry.Setup(ctx, cfg, log)
	if err != nil {
		return err
	}
	defer func() {
		// ctx is cancelled by the time this runs, so flush on a fresh one.
		flushCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := tel.Shutdown(flushCtx); err != nil {
			log.Error("flush telemetry", "error", err)
		}
	}()

	pool, err := store.OpenPostgres(ctx, cfg.DatabaseURL, cfg.StartupTimeout, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := migrate.Run(ctx, pool, migrations.Files, migrations.Dir, log)
	if err != nil {
		return err
	}
	log.Info("schema up to date", "migrationsApplied", applied)

	redisClient, err := store.OpenRedis(ctx, cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB, cfg.StartupTimeout, log)
	if err != nil {
		return err
	}
	defer func() { _ = redisClient.Close() }()

	compute, err := buildComputeRegistry(cfg, log)
	if err != nil {
		return err
	}

	knowledgeDeps, err := buildKnowledge(ctx, cfg, pool, log)
	if err != nil {
		return err
	}

	limiter := ratelimit.New(redisClient)
	agentDeps, err := buildAgent(ctx, cfg, pool, compute, knowledgeDeps, limiter, log)
	if err != nil {
		return err
	}

	keys := auth.NewStore(pool)
	handler := httpapi.NewRouter(httpapi.Deps{
		Config:        cfg,
		Log:           log,
		Metrics:       tel.MetricsHandler,
		Compute:       compute,
		Auth:          keys,
		Keys:          keys,
		Limiter:       limiter,
		Audit:         audit.NewRecorder(pool, log),
		Assistants:    assistant.NewStore(pool),
		Conversations: conversation.NewStore(pool, cfg.PersistPrompts),
		Knowledge:     knowledgeDeps,
		Agent:         agentDeps,
		Checkers: []httpapi.Checker{
			httpapi.CheckerFunc{DependencyName: "postgres", Probe: pool.Ping},
			httpapi.CheckerFunc{DependencyName: "redis", Probe: func(ctx context.Context) error {
				return redisClient.Ping(ctx).Err()
			}},
		},
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("control plane listening", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Info("shutdown signal received", "timeout", cfg.ShutdownTimeout.String())
	}

	// ctx is already cancelled here, so drain in-flight requests on a fresh one.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	log.Info("control plane stopped cleanly")
	return nil
}

// buildKnowledge wires the corpus: storage adapter, embedding provider,
// classification policy and the ingestion worker.
//
// The worker runs for as long as ctx lives, which is until the process is asked
// to stop. It is started here rather than in run() so everything the knowledge
// platform needs is assembled in one place.
func buildKnowledge(ctx context.Context, cfg config.Config, pool *pgxpool.Pool, log *slog.Logger) (httpapi.Knowledge, error) {
	policy, err := knowledge.NewPolicy(cfg.ClassificationLevels)
	if err != nil {
		return httpapi.Knowledge{}, fmt.Errorf("classification policy: %w", err)
	}
	plan, err := knowledge.NewChunkPlan(cfg.ChunkSize, cfg.ChunkOverlap)
	if err != nil {
		return httpapi.Knowledge{}, fmt.Errorf("chunk plan: %w", err)
	}
	objects, err := storage.NewLocal(cfg.StorageRoot)
	if err != nil {
		return httpapi.Knowledge{}, err
	}

	embedder := ollama.NewEmbedder(cfg.OllamaBaseURL, cfg.EmbeddingModel, cfg.EmbeddingDimensions, cfg.ComputeTimeout)
	documents := knowledge.NewStore(pool)

	// Ingestion never blocks startup: the compute plane may be down, and
	// documents simply wait as pending until it returns.
	worker := knowledge.NewWorker(documents, objects, embedder, plan,
		cfg.IngestionInterval, cfg.IngestionBatch, log)
	go worker.Run(ctx)

	log.Info("knowledge platform ready",
		"storage", objects.Name(), "root", cfg.StorageRoot,
		"classifications", policy.Levels(),
		"chunkSize", plan.Size, "chunkOverlap", plan.Overlap)

	return httpapi.Knowledge{
		Documents: documents,
		Storage:   objects,
		Embedder:  embedder,
		Policy:    policy,
	}, nil
}

// buildAgent wires the tool registry and the orchestrator.
//
// The registry is read from the database at startup: a registration naming an
// implementation this build does not have is refused here, where an operator can
// see it, rather than on somebody's first question.
func buildAgent(ctx context.Context, cfg config.Config, pool *pgxpool.Pool,
	compute *provider.Registry, knowledgeDeps httpapi.Knowledge,
	limiter *ratelimit.Limiter, log *slog.Logger) (httpapi.Agent, error) {

	policy, err := agent.NewPolicy(cfg.AgentMaxSteps, cfg.AgentTopK, cfg.AgentMinScore, cfg.AgentConflictMargin)
	if err != nil {
		return httpapi.Agent{}, err
	}

	runs := agent.NewStore(pool, cfg.PersistPrompts)
	registrations, err := runs.Tools(ctx)
	if err != nil {
		return httpapi.Agent{}, err
	}

	implementations := []tool.Implementation{
		tool.KnowledgeSearch{
			Documents: knowledgeDeps.Documents,
			Embedder:  knowledgeDeps.Embedder,
			Policy:    knowledgeDeps.Policy,
			TopK:      cfg.AgentTopK,
		},
		tool.ComputeHealth{Providers: func(ctx context.Context) (map[string]string, error) {
			statuses := make(map[string]string)
			for _, llm := range compute.Providers() {
				if err := llm.Health(ctx); err != nil {
					statuses[llm.Name()] = "unavailable"
					continue
				}
				statuses[llm.Name()] = "available"
			}
			return statuses, nil
		}},
		tool.PlatformTime{},
	}

	// Tool arguments are derived from user content, so they follow the same
	// prompt-logging policy as conversations.
	registry, err := tool.NewRegistry(registrations, implementations, limiter, runs, cfg.PersistPrompts)
	if err != nil {
		return httpapi.Agent{}, err
	}

	log.Info("agent ready",
		"tools", len(registrations), "maxSteps", policy.MaxSteps, "topK", policy.TopK,
		"minScore", policy.MinScore, "conflictMargin", policy.ConflictMargin)

	return httpapi.Agent{
		Orchestrator: &agent.Orchestrator{
			Retriever: knowledgeDeps.Documents,
			Embedder:  knowledgeDeps.Embedder,
			Completer: compute,
			Tools:     registry,
			Policy:    policy,
			Classes:   knowledgeDeps.Policy,
		},
		Runs:  runs,
		Tools: registry,
	}, nil
}

// runAPIKey mints the first key.
//
// Key creation over HTTP requires the admin:keys scope, which no key holds
// until one exists. Bootstrapping through the binary avoids the alternative:
// an environment-configured master credential that would outlive its purpose.
func runAPIKey(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: control-plane apikey create -name NAME -scopes %s", strings.Join(auth.KnownScopes, ","))
	}

	flags := flag.NewFlagSet("apikey create", flag.ContinueOnError)
	name := flags.String("name", "", "human-readable key name")
	scopes := flags.String("scopes", strings.Join(auth.KnownScopes, ","), "comma-separated scopes")
	company := flags.String("company", "", "company id this key is scoped to")
	department := flags.String("department", "", "department this key is scoped to")
	classification := flags.String("classification", "", "highest classification this key may read")
	rate := flags.Int("rate", 0, "requests per minute (defaults to DEFAULT_RATE_LIMIT_PER_MINUTE)")
	expires := flags.String("expires", "", "RFC3339 expiry timestamp")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("-name is required")
	}

	cfg, err := config.Load(os.Getenv, version)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, cancel := context.WithTimeout(context.Background(), cfg.StartupTimeout)
	defer cancel()

	pool, err := store.OpenPostgres(ctx, cfg.DatabaseURL, cfg.StartupTimeout, log)
	if err != nil {
		return err
	}
	defer pool.Close()

	// The schema may not exist yet on a first run.
	if _, err := migrate.Run(ctx, pool, migrations.Files, migrations.Dir, log); err != nil {
		return err
	}

	// An unset clearance falls to the column default rather than the top of the
	// ladder: a key should not silently gain access to everything.
	clearance := *classification
	if clearance != "" {
		policy, err := knowledge.NewPolicy(cfg.ClassificationLevels)
		if err != nil {
			return err
		}
		if clearance, err = policy.Normalise(clearance); err != nil {
			return err
		}
	}

	params := auth.CreateParams{
		Name:               *name,
		Scopes:             strings.Split(*scopes, ","),
		CompanyID:          *company,
		Department:         *department,
		MaxClassification:  clearance,
		RateLimitPerMinute: cfg.DefaultRateLimitPerMinute,
		CreatedBy:          "cli",
	}
	if *rate > 0 {
		params.RateLimitPerMinute = *rate
	}
	if *expires != "" {
		parsed, err := time.Parse(time.RFC3339, *expires)
		if err != nil {
			return fmt.Errorf("-expires must be an RFC3339 timestamp: %w", err)
		}
		params.ExpiresAt = &parsed
	}

	record, secret, err := auth.NewStore(pool).Create(ctx, params)
	if err != nil {
		return err
	}

	audit.NewRecorder(pool, log).Record(ctx, audit.Event{
		ActorID: "cli", Action: "api_key.created", ResourceType: "api_key", ResourceID: record.ID,
		CompanyID: record.CompanyID, Metadata: map[string]any{"name": record.Name, "scopes": record.Scopes, "source": "cli"},
	})

	// stdout carries the secret and nothing else, so it can be captured without
	// dragging log lines along. It is never written to the log.
	fmt.Printf("id        %s\nname      %s\nscopes    %s\nclearance %s\nrate      %d/min\nsecret    %s\n",
		record.ID, record.Name, strings.Join(record.Scopes, ","),
		record.MaxClassification, record.RateLimitPerMinute, secret)
	fmt.Fprintln(os.Stderr, "Store the secret now. It cannot be retrieved again.")
	return nil
}

// buildComputeRegistry wires the configured provider adapters and validates the
// routing table. It never probes the compute plane: VM5 may legitimately be
// down at startup, and that must not stop the Control Plane from serving
// (ARCHITECTURE-v1 section 9).
func buildComputeRegistry(cfg config.Config, log *slog.Logger) (*provider.Registry, error) {
	if cfg.ComputeProvider != "ollama" {
		return nil, fmt.Errorf("COMPUTE_PROVIDER %q has no adapter yet; only ollama is implemented", cfg.ComputeProvider)
	}

	routes, err := provider.ParseRoutes(cfg.ModelRoutes)
	if err != nil {
		return nil, err
	}
	registry, err := provider.NewRegistry(routes, cfg.DefaultModel, ollama.New(cfg.OllamaBaseURL, cfg.ComputeTimeout))
	if err != nil {
		return nil, err
	}

	log.Info("compute registry ready", "defaultModel", registry.DefaultModel(), "routes", registry.Routes())
	return registry, nil
}
