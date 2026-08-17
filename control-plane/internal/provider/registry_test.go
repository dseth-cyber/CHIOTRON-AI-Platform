package provider

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeLLM struct {
	name       string
	lastModel  string
	chatErr    error
	healthErr  error
	modelsList []Model
}

func (f *fakeLLM) Name() string                 { return f.name }
func (f *fakeLLM) Health(context.Context) error { return f.healthErr }
func (f *fakeLLM) Models(context.Context) ([]Model, error) {
	return f.modelsList, nil
}

func (f *fakeLLM) Chat(_ context.Context, req ChatRequest) (ChatResponse, error) {
	f.lastModel = req.Model
	if f.chatErr != nil {
		return ChatResponse{}, f.chatErr
	}
	return ChatResponse{Model: req.Model, Content: "ok"}, nil
}

// Upstream model names contain colons, so the provider/model split must happen
// on the first slash rather than on a colon.
func TestParseRoutesKeepsColonsInModelNames(t *testing.T) {
	routes, err := ParseRoutes(" default=ollama/qwen2.5:0.5b , fast=ollama/qwen2.5:0.5b-instruct ,")
	if err != nil {
		t.Fatalf("ParseRoutes() returned error: %v", err)
	}
	if len(routes) != 2 {
		t.Fatalf("ParseRoutes() returned %d routes, want 2 with the trailing blank dropped", len(routes))
	}
	if routes[0] != (Route{Logical: "default", Provider: "ollama", Model: "qwen2.5:0.5b"}) {
		t.Errorf("first route = %+v, want the colon preserved in the model name", routes[0])
	}
}

func TestParseRoutesRejectsBadSpecs(t *testing.T) {
	cases := map[string]string{
		"missing equals": "ollama/qwen2.5:0.5b",
		"missing slash":  "default=ollama",
		"empty field":    "default=/qwen2.5:0.5b",
		"empty spec":     "   ",
	}
	for name, spec := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseRoutes(spec); err == nil {
				t.Fatalf("ParseRoutes(%q) succeeded, want error", spec)
			}
		})
	}
}

// A typo in the routing table must stop the process at startup, not surface on
// a user's first request.
func TestNewRegistryValidatesRoutes(t *testing.T) {
	ollama := &fakeLLM{name: "ollama"}

	cases := map[string]struct {
		routes       []Route
		defaultModel string
		want         string
	}{
		"unknown provider": {
			routes:       []Route{{Logical: "default", Provider: "vllm", Model: "x"}},
			defaultModel: "default",
			want:         "unknown provider",
		},
		"duplicate logical": {
			routes: []Route{
				{Logical: "default", Provider: "ollama", Model: "a"},
				{Logical: "default", Provider: "ollama", Model: "b"},
			},
			defaultModel: "default",
			want:         "defined twice",
		},
		"default without route": {
			routes:       []Route{{Logical: "fast", Provider: "ollama", Model: "a"}},
			defaultModel: "default",
			want:         "has no route",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewRegistry(tc.routes, tc.defaultModel, ollama)
			if err == nil {
				t.Fatal("NewRegistry() succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestResolveFallsBackToDefault(t *testing.T) {
	registry := newTestRegistry(t, &fakeLLM{name: "ollama"})

	_, route, err := registry.Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\") returned error: %v", err)
	}
	if route.Logical != "default" {
		t.Errorf("Resolve(\"\") = %q, want the configured default", route.Logical)
	}

	if _, _, err := registry.Resolve("nope"); !errors.Is(err, ErrUnknownModel) {
		t.Errorf("Resolve(\"nope\") error = %v, want ErrUnknownModel", err)
	}
}

// Adapters must receive the upstream model name, never the logical id.
func TestChatSendsUpstreamModelName(t *testing.T) {
	llm := &fakeLLM{name: "ollama"}
	registry := newTestRegistry(t, llm)

	_, route, err := registry.Chat(context.Background(), "fast", ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}
	if llm.lastModel != "qwen2.5:0.5b-instruct" {
		t.Errorf("adapter received model %q, want the upstream name", llm.lastModel)
	}
	if route.Logical != "fast" {
		t.Errorf("route.Logical = %q, want fast", route.Logical)
	}
}

func TestChatPropagatesProviderFailure(t *testing.T) {
	llm := &fakeLLM{name: "ollama", chatErr: ErrUnavailable}
	registry := newTestRegistry(t, llm)

	if _, _, err := registry.Chat(context.Background(), "", ChatRequest{}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Chat() error = %v, want ErrUnavailable", err)
	}
}

func newTestRegistry(t *testing.T, providers ...LLM) *Registry {
	t.Helper()
	routes, err := ParseRoutes("default=ollama/qwen2.5:0.5b,fast=ollama/qwen2.5:0.5b-instruct")
	if err != nil {
		t.Fatalf("ParseRoutes() returned error: %v", err)
	}
	registry, err := NewRegistry(routes, "default", providers...)
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}
	return registry
}
