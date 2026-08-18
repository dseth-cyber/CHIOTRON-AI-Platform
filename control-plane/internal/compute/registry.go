package compute

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// Build assembles a routing registry from the database.
//
// A provider that will not build is skipped with its reason recorded rather
// than stopping the process: one misconfigured backend must not take the
// platform down, the same way losing VM5 does not (ARCHITECTURE-v1 section 9).
func (s *Store) Build(ctx context.Context, log *slog.Logger) (*provider.Registry, error) {
	providers, err := s.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	routes, err := s.ListRoutes(ctx)
	if err != nil {
		return nil, err
	}

	ceilings := make(map[string]string, len(providers))
	adapters := make([]provider.LLM, 0, len(providers))
	for _, record := range providers {
		if !record.Enabled {
			continue
		}
		adapter, err := s.Adapter(ctx, record)
		if err != nil {
			log.Error("build provider adapter", "provider", record.Slug, "error", err)
			_ = s.RecordCheck(ctx, record.Slug, "misconfigured", err.Error())
			continue
		}
		ceilings[record.Slug] = record.MaxClassification
		adapters = append(adapters, adapter)
	}

	parsed := make([]provider.Route, 0, len(routes))
	defaultLogical := ""
	for _, route := range routes {
		if !route.Enabled {
			continue
		}
		// A route whose provider failed to build is dropped here rather than
		// passed to NewRegistry, which would refuse the whole table for it.
		ceiling, ok := ceilings[route.ProviderSlug]
		if !ok {
			log.Warn("route skipped: provider unavailable",
				"logical", route.Logical, "provider", route.ProviderSlug)
			continue
		}
		parsed = append(parsed, provider.Route{
			Logical:           route.Logical,
			Provider:          route.ProviderSlug,
			Model:             route.UpstreamModel,
			MaxClassification: ceiling,
		})
		if route.IsDefault {
			defaultLogical = route.Logical
		}
	}

	if len(parsed) == 0 {
		return nil, fmt.Errorf("no usable model route is configured")
	}
	if defaultLogical == "" {
		// Falling back rather than failing: a platform that answers on a named
		// model but refuses every unnamed call is harder to diagnose than one
		// that picks the only route it has.
		defaultLogical = parsed[0].Logical
		log.Warn("no default route configured; using the first available",
			"logical", defaultLogical)
	}

	return provider.NewRegistry(parsed, defaultLogical, adapters...)
}

// Seed writes the environment's routing into the database the first time the
// platform starts against an empty providers table.
//
// It exists so an existing deployment keeps working after this migration
// without an operator having to re-enter what was already in its environment.
// It runs once: after that the database is the source of truth and the env vars
// are ignored, because two places to change the same thing is one too many.
func (s *Store) Seed(ctx context.Context, computeProvider, baseURL, routeSpec,
	defaultModel string, log *slog.Logger) error {

	existing, err := s.ListProviders(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	if computeProvider != KindOllama {
		return fmt.Errorf("cannot seed provider kind %q from the environment", computeProvider)
	}

	if _, err := s.CreateProvider(ctx, CreateParams{
		Slug: KindOllama, Name: "Local Ollama",
		Description: "Seeded from OLLAMA_BASE_URL on first start.",
		Kind:        KindOllama, BaseURL: baseURL,
		// The compute plane is on the private network and is the whole point of
		// keeping inference in-house, so it may see everything.
		MaxClassification: "restricted",
		TimeoutSeconds:    60, CreatedBy: "seed",
	}); err != nil {
		return fmt.Errorf("seed provider: %w", err)
	}

	routes, err := provider.ParseRoutes(routeSpec)
	if err != nil {
		return fmt.Errorf("seed routes: %w", err)
	}
	for _, route := range routes {
		if _, err := s.SaveRoute(ctx, RouteParams{
			Logical: route.Logical, ProviderSlug: route.Provider,
			UpstreamModel: route.Model, IsDefault: route.Logical == defaultModel,
			CreatedBy: "seed",
		}); err != nil {
			return fmt.Errorf("seed route %q: %w", route.Logical, err)
		}
	}

	log.Info("seeded provider registry from the environment",
		"provider", computeProvider, "routes", len(routes), "default", defaultModel)
	return nil
}
