// Package tool is the controlled registry of things an agent may do.
//
// Every tool declares the scope a caller must hold, is rate limited per key and
// per tool, and writes an execution record. Nothing here trusts the caller to
// have been checked upstream: the backend authorizes every tool call
// (ARCHITECTURE-v1 section 5).
package tool

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chiotron/ai-control-plane/internal/auth"
)

var (
	// ErrUnknownTool is a tool the registry has no implementation for.
	ErrUnknownTool = errors.New("unknown tool")
	// ErrNotPermitted is a tool the caller's credential may not use.
	ErrNotPermitted = errors.New("tool not permitted")
	// ErrThrottled is a tool the caller has called too often.
	ErrThrottled = errors.New("tool call rate exceeded")
	// ErrInvalidArguments is a call the tool refused before doing any work.
	ErrInvalidArguments = errors.New("invalid tool arguments")
)

// Registration is a tool as configured, independent of its implementation.
type Registration struct {
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	Kind              string `json:"kind"`
	RequiredScope     string `json:"requiredScope"`
	CompanyID         string `json:"companyId,omitempty"`
	MaxCallsPerMinute int    `json:"maxCallsPerMinute"`
}

// Result is what a tool produced. Content is what the model may read; Data is
// the structured form for a client.
type Result struct {
	Content string `json:"content"`
	Data    any    `json:"data,omitempty"`
}

// Invocation is one call, already authorized.
type Invocation struct {
	Caller    auth.Identity
	Arguments map[string]any
}

// Implementation is the behaviour behind a registration.
//
// A tool receives the caller's identity because its own authorization is not
// optional: `knowledge.search` must apply the caller's clearance, not the
// agent's.
type Implementation interface {
	Kind() string
	// Describe returns the argument contract, for a planner or a client to read.
	Describe() map[string]string
	// PrimaryArgument names the argument a caller's question maps onto, or the
	// empty string for a tool that takes none. Without it a caller has to know
	// each tool's parameter name, and an orchestrator guessing one name for all of
	// them silently fails against the rest.
	PrimaryArgument() string
	Invoke(ctx context.Context, call Invocation) (Result, error)
}

// Limiter throttles one subject. It is satisfied by the same Redis limiter the
// HTTP layer uses, so a tool cannot become a way around a key's request ceiling.
type Limiter interface {
	Permit(ctx context.Context, subject string, limit int) (bool, error)
}

// Observer receives tool outcomes for metrics. It is separate from Recorder
// because one writes a durable audit row and the other moves a counter, and a
// deployment may want either without the other.
type Observer interface {
	RecordToolCall(ctx context.Context, slug, outcome string, latency time.Duration)
}

// Recorder writes the tool-call audit trail.
type Recorder interface {
	RecordToolCall(ctx context.Context, call Record)
}

// Record is one execution, successful or not.
type Record struct {
	RunID     string
	Slug      string
	Caller    auth.Identity
	Arguments map[string]any
	Outcome   string
	Error     string
	Latency   time.Duration
}

const (
	OutcomeSuccess = "success"
	OutcomeDenied  = "denied"
	OutcomeFailure = "failure"
)

// Registry resolves a slug to an authorized, rate-limited, audited call.
type Registry struct {
	registrations   map[string]Registration
	ordered         []Registration
	implementations map[string]Implementation
	limiter         Limiter
	recorder        Recorder
	observer        Observer
	// redactArguments follows the prompt-logging policy: tool arguments are
	// derived from user content and are withheld when that is switched off.
	redactArguments bool
}

func NewRegistry(registrations []Registration, implementations []Implementation,
	limiter Limiter, recorder Recorder, observer Observer, recordArguments bool) (*Registry, error) {

	registry := &Registry{
		registrations:   make(map[string]Registration, len(registrations)),
		implementations: make(map[string]Implementation, len(implementations)),
		limiter:         limiter,
		recorder:        recorder,
		observer:        observer,
		redactArguments: !recordArguments,
	}
	for _, implementation := range implementations {
		registry.implementations[implementation.Kind()] = implementation
	}

	for _, registration := range registrations {
		if _, known := registry.implementations[registration.Kind]; !known {
			// A registration with no implementation would fail on first use, at
			// which point the operator has no idea why.
			return nil, fmt.Errorf("tool %q names unknown kind %q", registration.Slug, registration.Kind)
		}
		if registration.RequiredScope == "" {
			return nil, fmt.Errorf("tool %q declares no required scope", registration.Slug)
		}
		if _, duplicate := registry.registrations[registration.Slug]; duplicate {
			return nil, fmt.Errorf("tool %q is registered twice", registration.Slug)
		}
		registry.registrations[registration.Slug] = registration
		registry.ordered = append(registry.ordered, registration)
	}

	sort.Slice(registry.ordered, func(i, j int) bool {
		return registry.ordered[i].Slug < registry.ordered[j].Slug
	})
	return registry, nil
}

// Available lists the tools a caller may actually use: scope held and company
// matching. A client should not be shown a tool it cannot call.
func (r *Registry) Available(caller auth.Identity) []Registration {
	available := make([]Registration, 0, len(r.ordered))
	for _, registration := range r.ordered {
		if r.permitted(caller, registration) {
			available = append(available, registration)
		}
	}
	return available
}

// Describe returns the argument contract for a tool the caller may use.
func (r *Registry) Describe(caller auth.Identity, slug string) (map[string]string, error) {
	registration, ok := r.registrations[slug]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTool, slug)
	}
	if !r.permitted(caller, registration) {
		return nil, fmt.Errorf("%w: %q", ErrNotPermitted, slug)
	}
	return r.implementations[registration.Kind].Describe(), nil
}

// ArgumentsFor maps a question onto whatever argument the tool actually declares,
// so an orchestrator does not have to know each tool's parameter names.
func (r *Registry) ArgumentsFor(slug, question string) map[string]any {
	registration, ok := r.registrations[slug]
	if !ok {
		return map[string]any{}
	}
	primary := r.implementations[registration.Kind].PrimaryArgument()
	if primary == "" {
		return map[string]any{}
	}
	return map[string]any{primary: question}
}

func (r *Registry) permitted(caller auth.Identity, registration Registration) bool {
	if !caller.HasScope(registration.RequiredScope) {
		return false
	}
	// A company-scoped tool is only for that company; a tool with no company is
	// platform-wide.
	if registration.CompanyID != "" && registration.CompanyID != caller.CompanyID {
		return false
	}
	return true
}

// Invoke authorizes, throttles, executes and records one call.
//
// Every exit path writes a record: a denied or throttled call is exactly what an
// audit trail needs to show.
func (r *Registry) Invoke(ctx context.Context, runID, slug string, call Invocation) (Result, error) {
	started := time.Now()

	registration, ok := r.registrations[slug]
	if !ok {
		// Nothing to record against: the tool does not exist, so there is no
		// registration to attribute the attempt to beyond the caller.
		r.record(ctx, runID, slug, call, OutcomeDenied, "unknown tool", time.Since(started))
		return Result{}, fmt.Errorf("%w: %q", ErrUnknownTool, slug)
	}
	if !r.permitted(call.Caller, registration) {
		r.record(ctx, runID, slug, call, OutcomeDenied, "missing scope "+registration.RequiredScope, time.Since(started))
		return Result{}, fmt.Errorf("%w: %q needs %s", ErrNotPermitted, slug, registration.RequiredScope)
	}

	// The subject is the key and the tool together, so one noisy tool cannot
	// exhaust a caller's budget for the others.
	subject := "tool:" + slug + ":" + call.Caller.KeyID
	allowed, err := r.limiter.Permit(ctx, subject, registration.MaxCallsPerMinute)
	if err != nil {
		r.record(ctx, runID, slug, call, OutcomeFailure, "rate limiter unavailable", time.Since(started))
		return Result{}, fmt.Errorf("throttle %q: %w", slug, err)
	}
	if !allowed {
		r.record(ctx, runID, slug, call, OutcomeDenied, "rate limit exceeded", time.Since(started))
		return Result{}, fmt.Errorf("%w: %q allows %d calls a minute", ErrThrottled, slug, registration.MaxCallsPerMinute)
	}

	result, err := r.implementations[registration.Kind].Invoke(ctx, call)
	if err != nil {
		outcome := OutcomeFailure
		if errors.Is(err, ErrInvalidArguments) {
			outcome = OutcomeDenied
		}
		r.record(ctx, runID, slug, call, outcome, err.Error(), time.Since(started))
		return Result{}, err
	}

	r.record(ctx, runID, slug, call, OutcomeSuccess, "", time.Since(started))
	return result, nil
}

func (r *Registry) record(ctx context.Context, runID, slug string, call Invocation,
	outcome, reason string, latency time.Duration) {
	if r.observer != nil {
		r.observer.RecordToolCall(ctx, slug, outcome, latency)
	}
	if r.recorder == nil {
		return
	}
	arguments := call.Arguments
	if r.redactArguments {
		arguments = nil
	}
	r.recorder.RecordToolCall(ctx, Record{
		RunID: runID, Slug: slug, Caller: call.Caller, Arguments: arguments,
		Outcome: outcome, Error: reason, Latency: latency,
	})
}

// StringArgument reads a required string argument, trimmed.
func StringArgument(arguments map[string]any, name string) (string, error) {
	raw, present := arguments[name]
	if !present {
		return "", fmt.Errorf("%w: %q is required", ErrInvalidArguments, name)
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%w: %q must be a string", ErrInvalidArguments, name)
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("%w: %q must not be empty", ErrInvalidArguments, name)
	}
	return trimmed, nil
}

// IntArgument reads an optional integer argument. JSON numbers arrive as
// float64, which is why this is not a plain type assertion.
func IntArgument(arguments map[string]any, name string, fallback int) (int, error) {
	raw, present := arguments[name]
	if !present || raw == nil {
		return fallback, nil
	}
	switch value := raw.(type) {
	case float64:
		return int(value), nil
	case int:
		return value, nil
	default:
		return 0, fmt.Errorf("%w: %q must be a number", ErrInvalidArguments, name)
	}
}
