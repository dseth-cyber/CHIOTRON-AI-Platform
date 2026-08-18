package tool

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/mcp"
)

type stubCaller struct {
	result   mcp.CallResult
	err      error
	lastName string
	lastArgs map[string]any
	calls    int
}

func (s *stubCaller) Slug() string { return "erp" }

func (s *stubCaller) Call(_ context.Context, name string, arguments map[string]any) (mcp.CallResult, error) {
	s.calls++
	s.lastName = name
	s.lastArgs = arguments
	return s.result, s.err
}

func remoteTool(caller Caller) Remote {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string", "description": "What to look for."},
			"limit": map[string]any{"type": "integer"},
		},
		"required": []any{"query"},
	}
	return Remote{
		Client: caller, RemoteName: "search", Slug: "erp.search",
		Description: "Searches ERP records.", Schema: schema,
		Primary: InferPrimaryArgument(schema),
	}
}

// Each remote tool needs its own kind, or two servers offering the same tool
// name would collide in one registry.
func TestRemoteKindIsNamespaced(t *testing.T) {
	first := remoteTool(&stubCaller{})
	second := first
	second.Slug = "crm.search"

	if first.Kind() == second.Kind() {
		t.Errorf("two servers produced the same kind %q", first.Kind())
	}
	if !strings.HasPrefix(first.Kind(), "mcp:") {
		t.Errorf("kind = %q, want an mcp prefix", first.Kind())
	}
}

func TestRemoteDescribesTheRemoteSchema(t *testing.T) {
	described := remoteTool(&stubCaller{}).Describe()

	if !strings.Contains(described["query"], "required") {
		t.Errorf("query described as %q, want it marked required", described["query"])
	}
	if !strings.Contains(described["limit"], "optional") {
		t.Errorf("limit described as %q, want it marked optional", described["limit"])
	}
	if !strings.Contains(described["query"], "What to look for") {
		t.Errorf("query description lost the remote text: %q", described["query"])
	}
}

// A malformed call still crosses a trust boundary and still costs a rate-limit
// slot, so it is refused before it leaves the platform.
func TestRemoteValidatesBeforeCalling(t *testing.T) {
	caller := &stubCaller{}
	remote := remoteTool(caller)

	_, err := remote.Invoke(context.Background(), Invocation{Arguments: map[string]any{"limit": float64(1)}})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("error = %v, want ErrInvalidArguments", err)
	}
	if caller.calls != 0 {
		t.Error("the call was forwarded despite failing validation")
	}
	// The message lands in an audit row an operator reads, so it must name the
	// condition once.
	if strings.Count(err.Error(), "invalid tool arguments") != 1 {
		t.Errorf("error %q repeats the condition", err)
	}
	if !strings.Contains(err.Error(), `"query" is required`) {
		t.Errorf("error %q does not name the missing argument", err)
	}
}

func TestRemoteForwardsAValidCall(t *testing.T) {
	caller := &stubCaller{result: mcp.CallResult{Content: []mcp.Content{{Type: "text", Text: "3 invoices"}}}}
	remote := remoteTool(caller)

	result, err := remote.Invoke(context.Background(), Invocation{
		Arguments: map[string]any{"query": "unpaid invoices"},
	})
	if err != nil {
		t.Fatalf("Invoke() returned error: %v", err)
	}
	if caller.lastName != "search" {
		t.Errorf("remote name = %q, want the server's own tool name", caller.lastName)
	}
	if result.Content != "3 invoices" {
		t.Errorf("content = %q, want the rendered result", result.Content)
	}
}

// A tool reporting its own failure is not a platform failure, but it must not be
// returned as a success either.
func TestRemoteSurfacesToolReportedErrors(t *testing.T) {
	caller := &stubCaller{result: mcp.CallResult{
		IsError: true,
		Content: []mcp.Content{{Type: "text", Text: "customer not found"}},
	}}

	result, err := remoteTool(caller).Invoke(context.Background(), Invocation{
		Arguments: map[string]any{"query": "x"},
	})
	if err == nil {
		t.Fatal("Invoke() returned no error for a tool-reported failure")
	}
	// The text still comes back, so a model can react to what went wrong.
	if !strings.Contains(result.Content, "customer not found") {
		t.Errorf("content = %q, want the tool's own message", result.Content)
	}
}

// A single required string is the unambiguous case.
func TestInferPrimaryArgumentPrefersTheLoneRequiredString(t *testing.T) {
	primary := InferPrimaryArgument(map[string]any{
		"properties": map[string]any{
			"needle": map[string]any{"type": "string"},
			"limit":  map[string]any{"type": "integer"},
		},
		"required": []any{"needle", "limit"},
	})
	if primary != "needle" {
		t.Errorf("primary = %q, want needle", primary)
	}
}

func TestInferPrimaryArgumentFallsBackToConvention(t *testing.T) {
	primary := InferPrimaryArgument(map[string]any{
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
			"other": map[string]any{"type": "string"},
		},
	})
	if primary != "query" {
		t.Errorf("primary = %q, want query", primary)
	}
}

// Guessing wrong would send a question to an argument that means something else,
// so an ambiguous schema yields nothing and the tool stays callable only with
// explicit arguments.
func TestInferPrimaryArgumentGivesUpWhenAmbiguous(t *testing.T) {
	primary := InferPrimaryArgument(map[string]any{
		"properties": map[string]any{
			"firstName": map[string]any{"type": "string"},
			"lastName":  map[string]any{"type": "string"},
		},
	})
	if primary != "" {
		t.Errorf("primary = %q, want none for an ambiguous schema", primary)
	}
	if primary := InferPrimaryArgument(map[string]any{}); primary != "" {
		t.Errorf("primary = %q, want none for an empty schema", primary)
	}
}
