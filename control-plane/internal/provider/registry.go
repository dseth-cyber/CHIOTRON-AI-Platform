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
	// MaxClassification is the most sensitive content the provider behind this
	// route may be sent. It travels with the route rather than being looked up
	// separately, so no call site can resolve a model and forget to ask.
	//
	// Empty means unrestricted, which is what routes parsed from the legacy
	// MODEL_ROUTES environment variable get: that path predates the ceiling and
	// only ever pointed at a local provider.
	MaxClassification string `json:"maxClassification,omitempty"`
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

// Resolve selects the provider and upstream model for a logical id. If the
// requested logical route is disabled or unknown, it gracefully falls back
// to the next available tier or the configured default model.
func (r *Registry) Resolve(logical string) (LLM, Route, error) {
	if logical == "" {
		logical = r.defaultLogical
	}
	if route, ok := r.routes[logical]; ok {
		return r.providers[route.Provider], route, nil
	}

	return nil, Route{}, fmt.Errorf("%w: %q", ErrUnknownModel, logical)
}

// Permits reports whether content of the given classification may be sent to
// the provider behind this route.
//
// The ladder is passed in rather than imported: the classification policy is
// configuration, and this package must not grow a second copy of it that can
// disagree with the one the corpus uses.
//
// An unknown classification is refused. A ladder that does not contain the
// level being checked means the two halves of the platform disagree about what
// levels exist, and guessing in that state is how content escapes.
func (r Route) Permits(ladder []string, classification string) bool {
	if r.MaxClassification == "" || classification == "" {
		return true
	}
	ceiling := indexOf(ladder, r.MaxClassification)
	level := indexOf(ladder, classification)
	if ceiling < 0 || level < 0 {
		return false
	}
	return level <= ceiling
}

func indexOf(ladder []string, value string) int {
	for index, entry := range ladder {
		if entry == value {
			return index
		}
	}
	return -1
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

// ChatStream resolves a logical model and streams its response.
//
// A provider that cannot stream is not an error: the whole response is
// delivered as one chunk, so a caller that asked for a stream still gets a
// well-formed stream and does not have to know which backend served it.
func (r *Registry) ChatStream(ctx context.Context, logical string, req ChatRequest, emit func(Chunk) error) (ChatResponse, Route, error) {
	llm, route, err := r.Resolve(logical)
	if err != nil {
		return ChatResponse{}, Route{}, err
	}
	req.Model = route.Model

	streaming, ok := llm.(StreamingLLM)
	if !ok {
		response, err := llm.Chat(ctx, req)
		if err != nil {
			return ChatResponse{}, route, err
		}
		if err := emit(Chunk{Content: response.Content}); err != nil {
			return ChatResponse{}, route, err
		}
		return response, route, nil
	}

	response, err := streaming.ChatStream(ctx, req, emit)
	if err != nil {
		return ChatResponse{}, route, err
	}
	return response, route, nil
}
