package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

// Searcher is the retrieval surface the knowledge tool needs. It is an interface
// so the tool can be exercised without a database.
type Searcher interface {
	Search(ctx context.Context, query string, embedding []float32, access knowledge.Access, limit int) ([]knowledge.Hit, error)
}

// Embedder vectorises the query text.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// KnowledgeSearch retrieves from the corpus.
//
// It derives the access predicates from the caller inside the tool, not from
// whatever the agent passes: an agent must not be able to widen the clearance of
// the person it is acting for.
type KnowledgeSearch struct {
	Documents Searcher
	Embedder  Embedder
	Policy    knowledge.Policy
	// TopK caps how many chunks one call returns.
	TopK int
}

func (KnowledgeSearch) Kind() string { return "knowledge.search" }

func (KnowledgeSearch) PrimaryArgument() string { return "query" }

func (KnowledgeSearch) Describe() map[string]string {
	return map[string]string{
		"query": "string, required. What to look for in the document corpus.",
		"limit": "number, optional. How many chunks to return, capped by policy.",
	}
}

func (k KnowledgeSearch) Invoke(ctx context.Context, call Invocation) (Result, error) {
	query, err := StringArgument(call.Arguments, "query")
	if err != nil {
		return Result{}, err
	}
	limit, err := IntArgument(call.Arguments, "limit", k.TopK)
	if err != nil {
		return Result{}, err
	}
	if limit <= 0 || limit > k.TopK {
		limit = k.TopK
	}

	readable, err := k.Policy.Readable(call.Caller.MaxClassification)
	if err != nil {
		return Result{}, err
	}
	access := knowledge.Access{
		CompanyID:       call.Caller.CompanyID,
		Department:      call.Caller.Department,
		Classifications: readable,
	}

	embeddings, err := k.Embedder.Embed(ctx, []string{query})
	if err != nil {
		return Result{}, fmt.Errorf("embed query: %w", err)
	}
	hits, err := k.Documents.Search(ctx, query, embeddings[0], access, limit)
	if err != nil {
		return Result{}, err
	}

	return Result{Content: renderHits(hits), Data: hits}, nil
}

// renderHits is what the model reads. Each passage is numbered so the model has
// something concrete to cite.
func renderHits(hits []knowledge.Hit) string {
	if len(hits) == 0 {
		return "No matching passages."
	}
	var builder strings.Builder
	for i, hit := range hits {
		fmt.Fprintf(&builder, "[%d] %s (part %d)\n%s\n\n", i+1, hit.DocumentTitle, hit.Ordinal+1, hit.Content)
	}
	return strings.TrimSpace(builder.String())
}

// ComputeHealth reports provider reachability, so an agent can say "the model
// plane is down" rather than simply failing.
type ComputeHealth struct {
	Providers func(ctx context.Context) (map[string]string, error)
}

func (ComputeHealth) Kind() string { return "compute.health" }

func (ComputeHealth) PrimaryArgument() string { return "" }

func (ComputeHealth) Describe() map[string]string { return map[string]string{} }

func (c ComputeHealth) Invoke(ctx context.Context, _ Invocation) (Result, error) {
	statuses, err := c.Providers(ctx)
	if err != nil {
		return Result{}, err
	}

	parts := make([]string, 0, len(statuses))
	for name, status := range statuses {
		parts = append(parts, name+": "+status)
	}
	if len(parts) == 0 {
		return Result{Content: "No compute providers are registered.", Data: statuses}, nil
	}
	return Result{Content: strings.Join(parts, ", "), Data: statuses}, nil
}

// PlatformTime answers what the platform thinks the time is.
//
// It looks trivial, and it is: a model has no clock, so without this it invents
// one. Being the smallest possible tool also makes it the honest test of the
// registry's authorization and audit path.
type PlatformTime struct {
	Now func() time.Time
}

func (PlatformTime) Kind() string { return "platform.time" }

func (PlatformTime) PrimaryArgument() string { return "" }

func (PlatformTime) Describe() map[string]string { return map[string]string{} }

func (p PlatformTime) Invoke(context.Context, Invocation) (Result, error) {
	now := time.Now
	if p.Now != nil {
		now = p.Now
	}
	stamp := now().UTC().Format(time.RFC3339)
	return Result{Content: "Current platform time is " + stamp + " (UTC).", Data: stamp}, nil
}
