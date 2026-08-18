package tool

import (
	"context"
	"errors"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/auth"
)

type stubImplementation struct {
	kind    string
	primary string
	result  Result
	err     error
	calls   int
}

func (s *stubImplementation) Kind() string            { return s.kind }
func (s *stubImplementation) PrimaryArgument() string { return s.primary }
func (s *stubImplementation) Describe() map[string]string {
	return map[string]string{"query": "string"}
}

func (s *stubImplementation) Invoke(context.Context, Invocation) (Result, error) {
	s.calls++
	if s.err != nil {
		return Result{}, s.err
	}
	return s.result, nil
}

type stubLimiter struct {
	allow    bool
	err      error
	subjects []string
	limits   []int
}

func (s *stubLimiter) Permit(_ context.Context, subject string, limit int) (bool, error) {
	s.subjects = append(s.subjects, subject)
	s.limits = append(s.limits, limit)
	if s.err != nil {
		return false, s.err
	}
	return s.allow, nil
}

type stubRecorder struct{ records []Record }

func (s *stubRecorder) RecordToolCall(_ context.Context, call Record) {
	s.records = append(s.records, call)
}

func testCaller(scopes ...string) auth.Identity {
	return auth.Identity{KeyID: "key-1", Scopes: scopes, MaxClassification: "internal"}
}

func testRegistry(t *testing.T, limiter *stubLimiter, recorder *stubRecorder,
	implementation Implementation, registrations ...Registration) *Registry {
	t.Helper()
	if len(registrations) == 0 {
		registrations = []Registration{{
			Slug: "demo.tool", Name: "Demo", Kind: "demo",
			RequiredScope: "models:read", MaxCallsPerMinute: 5,
		}}
	}
	registry, err := NewRegistry(registrations, []Implementation{implementation}, limiter, recorder, nil, true)
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}
	return registry
}

// A registration with no implementation would fail on first use, when the
// operator has no idea why.
func TestNewRegistryRejectsBadRegistrations(t *testing.T) {
	implementation := &stubImplementation{kind: "demo"}

	cases := map[string][]Registration{
		"unknown kind": {{Slug: "a", Kind: "missing", RequiredScope: "models:read"}},
		"no scope":     {{Slug: "a", Kind: "demo"}},
		"duplicate slug": {
			{Slug: "a", Kind: "demo", RequiredScope: "models:read"},
			{Slug: "a", Kind: "demo", RequiredScope: "models:read"},
		},
	}
	for name, registrations := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := NewRegistry(registrations, []Implementation{implementation},
				&stubLimiter{allow: true}, &stubRecorder{}, nil, true)
			if err == nil {
				t.Fatal("NewRegistry() succeeded, want error")
			}
		})
	}
}

func TestInvokeRequiresTheDeclaredScope(t *testing.T) {
	implementation := &stubImplementation{kind: "demo"}
	recorder := &stubRecorder{}
	registry := testRegistry(t, &stubLimiter{allow: true}, recorder, implementation)

	_, err := registry.Invoke(context.Background(), "", "demo.tool", Invocation{
		Caller: testCaller("chat:completions"),
	})
	if !errors.Is(err, ErrNotPermitted) {
		t.Fatalf("error = %v, want ErrNotPermitted", err)
	}
	if implementation.calls != 0 {
		t.Error("the tool ran despite the caller lacking its scope")
	}
	// A denied call is exactly what an audit trail needs to show.
	if len(recorder.records) != 1 || recorder.records[0].Outcome != OutcomeDenied {
		t.Errorf("records = %+v, want one denied record", recorder.records)
	}
}

func TestInvokeThrottlesPerToolPerKey(t *testing.T) {
	implementation := &stubImplementation{kind: "demo"}
	limiter := &stubLimiter{allow: false}
	recorder := &stubRecorder{}
	registry := testRegistry(t, limiter, recorder, implementation)

	_, err := registry.Invoke(context.Background(), "", "demo.tool", Invocation{Caller: testCaller("models:read")})
	if !errors.Is(err, ErrThrottled) {
		t.Fatalf("error = %v, want ErrThrottled", err)
	}
	// One noisy tool must not exhaust a caller's budget for the others, so the
	// subject names both.
	if len(limiter.subjects) != 1 || limiter.subjects[0] != "tool:demo.tool:key-1" {
		t.Errorf("limiter subject = %v, want the tool and the key", limiter.subjects)
	}
	if limiter.limits[0] != 5 {
		t.Errorf("limit = %d, want the registration's 5", limiter.limits[0])
	}
	if recorder.records[0].Outcome != OutcomeDenied {
		t.Errorf("outcome = %q, want denied", recorder.records[0].Outcome)
	}
}

// A limiter outage must not let tool calls through unmetered.
func TestInvokeFailsClosedWhenTheLimiterIsDown(t *testing.T) {
	implementation := &stubImplementation{kind: "demo"}
	registry := testRegistry(t, &stubLimiter{err: errors.New("redis down")}, &stubRecorder{}, implementation)

	if _, err := registry.Invoke(context.Background(), "", "demo.tool",
		Invocation{Caller: testCaller("models:read")}); err == nil {
		t.Fatal("Invoke() succeeded with the limiter down, want an error")
	}
	if implementation.calls != 0 {
		t.Error("the tool ran while the limiter was unavailable")
	}
}

func TestInvokeRecordsSuccess(t *testing.T) {
	implementation := &stubImplementation{kind: "demo", result: Result{Content: "ok"}}
	recorder := &stubRecorder{}
	registry := testRegistry(t, &stubLimiter{allow: true}, recorder, implementation)

	result, err := registry.Invoke(context.Background(), "run-1", "demo.tool", Invocation{
		Caller: testCaller("models:read"), Arguments: map[string]any{"query": "hello"},
	})
	if err != nil {
		t.Fatalf("Invoke() returned error: %v", err)
	}
	if result.Content != "ok" {
		t.Errorf("content = %q, want ok", result.Content)
	}
	record := recorder.records[0]
	if record.Outcome != OutcomeSuccess || record.RunID != "run-1" || record.Slug != "demo.tool" {
		t.Errorf("record = %+v, want a successful call attributed to the run", record)
	}
	if record.Arguments["query"] != "hello" {
		t.Errorf("arguments = %v, want them recorded", record.Arguments)
	}
}

// Tool arguments are derived from user content, so they follow the same
// prompt-logging policy as conversations.
func TestArgumentsAreWithheldWhenPromptLoggingIsOff(t *testing.T) {
	implementation := &stubImplementation{kind: "demo", result: Result{Content: "ok"}}
	recorder := &stubRecorder{}
	registry, err := NewRegistry([]Registration{{
		Slug: "demo.tool", Kind: "demo", RequiredScope: "models:read", MaxCallsPerMinute: 5,
	}}, []Implementation{implementation}, &stubLimiter{allow: true}, recorder, nil, false)
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}

	if _, err := registry.Invoke(context.Background(), "", "demo.tool", Invocation{
		Caller: testCaller("models:read"), Arguments: map[string]any{"query": "salary data"},
	}); err != nil {
		t.Fatalf("Invoke() returned error: %v", err)
	}
	if recorder.records[0].Arguments != nil {
		t.Errorf("arguments = %v, want them withheld", recorder.records[0].Arguments)
	}
}

func TestUnknownToolIsRefusedAndRecorded(t *testing.T) {
	recorder := &stubRecorder{}
	registry := testRegistry(t, &stubLimiter{allow: true}, recorder, &stubImplementation{kind: "demo"})

	if _, err := registry.Invoke(context.Background(), "", "nope",
		Invocation{Caller: testCaller("models:read")}); !errors.Is(err, ErrUnknownTool) {
		t.Fatalf("error = %v, want ErrUnknownTool", err)
	}
	if len(recorder.records) != 1 {
		t.Error("an attempt on an unknown tool was not recorded")
	}
}

// A client should not be shown a tool it cannot call.
func TestAvailableFiltersByScopeAndCompany(t *testing.T) {
	implementation := &stubImplementation{kind: "demo"}
	registry := testRegistry(t, &stubLimiter{allow: true}, &stubRecorder{}, implementation,
		Registration{Slug: "open.tool", Kind: "demo", RequiredScope: "models:read", MaxCallsPerMinute: 5},
		Registration{Slug: "scoped.tool", Kind: "demo", RequiredScope: "knowledge:read", MaxCallsPerMinute: 5},
		Registration{Slug: "acme.tool", Kind: "demo", RequiredScope: "models:read", CompanyID: "acme", MaxCallsPerMinute: 5},
	)

	available := registry.Available(testCaller("models:read"))
	if len(available) != 1 || available[0].Slug != "open.tool" {
		t.Errorf("available = %+v, want only open.tool", available)
	}

	withCompany := auth.Identity{KeyID: "k", Scopes: []string{"models:read"}, CompanyID: "acme"}
	if got := registry.Available(withCompany); len(got) != 2 {
		t.Errorf("available for acme = %+v, want the platform and the company tool", got)
	}
}

// An orchestrator cannot know each tool's parameter names, and guessing one name
// for all of them silently fails against the rest.
func TestArgumentsForUsesTheToolsOwnParameterName(t *testing.T) {
	implementation := &stubImplementation{kind: "demo", primary: "term"}
	registry := testRegistry(t, &stubLimiter{allow: true}, &stubRecorder{}, implementation)

	arguments := registry.ArgumentsFor("demo.tool", "what is related to keys")
	if arguments["term"] != "what is related to keys" {
		t.Errorf("arguments = %v, want the question under the declared name", arguments)
	}
	if _, wrong := arguments["query"]; wrong {
		t.Errorf("arguments = %v, want no assumed parameter name", arguments)
	}
}

// A tool that takes no arguments must be called with none, not with a question it
// would reject.
func TestArgumentsForOmitsArgumentsWhenToolTakesNone(t *testing.T) {
	implementation := &stubImplementation{kind: "demo", primary: ""}
	registry := testRegistry(t, &stubLimiter{allow: true}, &stubRecorder{}, implementation)

	if arguments := registry.ArgumentsFor("demo.tool", "anything"); len(arguments) != 0 {
		t.Errorf("arguments = %v, want none", arguments)
	}
}

func TestArgumentsForUnknownToolIsEmpty(t *testing.T) {
	registry := testRegistry(t, &stubLimiter{allow: true}, &stubRecorder{}, &stubImplementation{kind: "demo"})

	if arguments := registry.ArgumentsFor("nope", "anything"); len(arguments) != 0 {
		t.Errorf("arguments = %v, want none for an unknown tool", arguments)
	}
}

func TestStringArgument(t *testing.T) {
	if _, err := StringArgument(map[string]any{}, "query"); !errors.Is(err, ErrInvalidArguments) {
		t.Error("a missing argument was accepted")
	}
	if _, err := StringArgument(map[string]any{"query": 42}, "query"); !errors.Is(err, ErrInvalidArguments) {
		t.Error("a non-string argument was accepted")
	}
	if _, err := StringArgument(map[string]any{"query": "   "}, "query"); !errors.Is(err, ErrInvalidArguments) {
		t.Error("a blank argument was accepted")
	}
	value, err := StringArgument(map[string]any{"query": "  hello  "}, "query")
	if err != nil || value != "hello" {
		t.Errorf("StringArgument() = %q, %v; want hello and no error", value, err)
	}
}

// JSON numbers arrive as float64, which is why this is not a type assertion.
func TestIntArgument(t *testing.T) {
	value, err := IntArgument(map[string]any{"limit": float64(7)}, "limit", 3)
	if err != nil || value != 7 {
		t.Errorf("IntArgument() = %d, %v; want 7", value, err)
	}
	if value, _ := IntArgument(map[string]any{}, "limit", 3); value != 3 {
		t.Errorf("IntArgument() = %d, want the fallback 3", value)
	}
	if _, err := IntArgument(map[string]any{"limit": "many"}, "limit", 3); !errors.Is(err, ErrInvalidArguments) {
		t.Error("a non-numeric argument was accepted")
	}
}
