// Command api runs the CHIOTRON AI Control Plane.
//
// It owns configuration, its own database schema and the HTTP surface the
// portal and future Gateway build on. It never talks to a model provider on
// behalf of a browser: users reach compute only through this plane
// (ARCHITECTURE-v1 sections 1 and 5).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chiotron/ai-control-plane/internal/config"
	"github.com/chiotron/ai-control-plane/internal/httpapi"
	"github.com/chiotron/ai-control-plane/internal/migrate"
	"github.com/chiotron/ai-control-plane/internal/migrations"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/provider/ollama"
	"github.com/chiotron/ai-control-plane/internal/store"
	"github.com/chiotron/ai-control-plane/internal/telemetry"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := run(); err != nil {
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

	handler := httpapi.NewRouter(httpapi.Deps{
		Config:  cfg,
		Log:     log,
		Metrics: tel.MetricsHandler,
		Compute: compute,
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
	if cfg.DevUnauthenticatedChat {
		log.Warn("POST /api/v1/chat/completions is exposed without authentication",
			"setting", "DEV_UNAUTHENTICATED_CHAT",
			"action", "turn this off anywhere real users can reach the service")
	}
	return registry, nil
}
