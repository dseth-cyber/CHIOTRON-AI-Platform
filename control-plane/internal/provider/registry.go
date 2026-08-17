package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Route maps a logical model id to a provider and the upstream model name.
//
// Logical ids are what callers ask for; upstream names and provider URLs are
// configuration, not application constants (ARCHITECTURE-v1 section 4). That
// is what lets a deployment move `default` from Ollama to vLLM without
// touching orchestration code.
type Route struct {
	Logical  string `json:"logical"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// Registry is the model registry and router.
type Registry struct {
	providers      map[string]LLM
	routes         map[string]Route
	defaultLogical string
}

// ParseRoutes reads a `logical=provider/model` list, for example
//
//	default=ollama/qwen2.5:0.5b,fast=ollama/qwen2.5:0.5b
//
// The provider and model are split on the first slash so upstream names may
// contain colons, as Ollama's do.
func ParseRoutes(spec string) ([]Route, error) {
	var routes []Route
	for _, entry := range strings.Split(spec, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		logical, target, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, fmt.Errorf("model route %q must be written logical=provider/model", entry)
		}
		providerName, model, ok := strings.Cut(target, "/")
		if !ok {
			return nil, fmt.Errorf("model route %q must name a provider and a model as provider/model", entry)
		}

		route := Route{
			Logical:  strings.TrimSpace(logical),
			Provider: strings.TrimSpace(providerName),
			Model:    strings.TrimSpace(model),
		}
		if route.Logical == "" || route.Provider == "" || route.Model == "" {
			return nil, fmt.Errorf("model route %q has an empty field", entry)
		}
		routes = append(routes, route)
	}
	if len(routes) == 0 {
		return nil, fmt.Errorf("no model routes configured")
	}
	return routes, nil
}

// NewRegistry validates the routing table against the registered providers so
// a typo fails at startup rather than on a user's first request.
func NewRegistry(routes []Route, defaultLogical string, providers ...LLM) (*Registry, error) {
	registry := &Registry{
		providers:      make(map[string]LLM, len(providers)),
		routes:         make(map[string]Route, len(routes)),
		defaultLogical: defaultLogical,
	}
	for _, p := range providers {
		if _, duplicate := registry.providers[p.Name()]; duplicate {
			return nil, fmt.Errorf("provider %q registered twice", p.Name())
		}
		registry.providers[p.Name()] = p
	}

	for _, route := range routes {
		if _, known := registry.providers[route.Provider]; !known {
			return nil, fmt.Errorf("model route %q names unknown provider %q", route.Logical, route.Provider)
		}
		if _, duplicate := registry.routes[route.Logical]; duplicate {
			return nil, fmt.Errorf("model route %q is defined twice", route.Logical)
		}
		registry.routes[route.Logical] = route
	}

	if _, ok := registry.routes[defaultLogical]; !ok {
		return nil, fmt.Errorf("default model %q has no route", defaultLogical)
	}
	return registry, nil
}

// Resolve selects the provider and upstream model for a logical id. An empty
// id resolves to the configured default.
func (r *Registry) Resolve(logical string) (LLM, Route, error) {
	if logical == "" {
		logical = r.defaultLogical
	}
	route, ok := r.routes[logical]
	if !ok {
		return nil, Route{}, fmt.Errorf("%w: %q", ErrUnknownModel, logical)
	}
	return r.providers[route.Provider], route, nil
}

// DefaultModel returns the logical id used when a caller names no model.
func (r *Registry) DefaultModel() string { return r.defaultLogical }

// Routes returns the routing table sorted by logical id for stable output.
func (r *Registry) Routes() []Route {
	routes := make([]Route, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, route)
	}
	sort.Slice(routes, func(i, j int) bool { return routes[i].Logical < routes[j].Logical })
	return routes
}

// Providers returns every registered provider sorted by name.
func (r *Registry) Providers() []LLM {
	providers := make([]LLM, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name() < providers[j].Name() })
	return providers
}

// Chat resolves a logical model and forwards the request to its provider.
func (r *Registry) Chat(ctx context.Context, logical string, req ChatRequest) (ChatResponse, Route, error) {
	llm, route, err := r.Resolve(logical)
	if err != nil {
		return ChatResponse{}, Route{}, err
	}
	req.Model = route.Model

	response, err := llm.Chat(ctx, req)
	if err != nil {
		return ChatResponse{}, route, err
	}
	return response, route, nil
}
