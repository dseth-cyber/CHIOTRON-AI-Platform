package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/graph"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

// Grapher is the traversal surface the tool needs, which is the GraphProvider
// contract narrowed to what this tool actually uses.
type Grapher interface {
	Search(ctx context.Context, term string, access graph.Access, limit int) ([]graph.Node, error)
	Neighbours(ctx context.Context, seeds []string, traversal graph.Traversal, access graph.Access) (graph.Subgraph, error)
}

// GraphNeighbours finds what a term is related to.
//
// Like knowledge.search it derives the caller's clearance inside the tool. A
// traversal is an especially sharp case: an unfiltered walk turns one readable
// node into a path to everything it touches.
type GraphNeighbours struct {
	Graph     Grapher
	Policy    knowledge.Policy
	Traversal graph.Traversal
}

func (GraphNeighbours) Kind() string { return "graph.neighbours" }

func (GraphNeighbours) PrimaryArgument() string { return "term" }

func (GraphNeighbours) Describe() map[string]string {
	return map[string]string{
		"term":  "string, required. The entity or acronym to start from.",
		"depth": "number, optional. How many hops to walk, capped by policy.",
	}
}

func (g GraphNeighbours) Invoke(ctx context.Context, call Invocation) (Result, error) {
	term, err := StringArgument(call.Arguments, "term")
	if err != nil {
		return Result{}, err
	}
	depth, err := IntArgument(call.Arguments, "depth", g.Traversal.Depth)
	if err != nil {
		return Result{}, err
	}
	if depth <= 0 || depth > g.Traversal.Depth {
		depth = g.Traversal.Depth
	}

	readable, err := g.Policy.Readable(call.Caller.MaxClassification)
	if err != nil {
		return Result{}, err
	}
	access := graph.Access{
		CompanyID:       call.Caller.CompanyID,
		Department:      call.Caller.Department,
		Classifications: readable,
	}

	seeds, err := g.Graph.Search(ctx, term, access, 5)
	if err != nil {
		return Result{}, err
	}
	if len(seeds) == 0 {
		return Result{Content: fmt.Sprintf("Nothing in the graph matches %q.", term), Data: graph.Subgraph{}}, nil
	}

	ids := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		ids = append(ids, seed.ID)
	}
	traversal, err := graph.NewTraversal(depth, g.Traversal.MaxNodes, g.Traversal.Relations)
	if err != nil {
		return Result{}, err
	}

	subgraph, err := g.Graph.Neighbours(ctx, ids, traversal, access)
	if err != nil {
		return Result{}, err
	}
	return Result{Content: renderSubgraph(term, subgraph), Data: subgraph}, nil
}

// renderSubgraph is what the model reads. It names the documents a relationship
// came from, so a claim built on the graph can still be cited.
func renderSubgraph(term string, subgraph graph.Subgraph) string {
	if len(subgraph.Nodes) == 0 {
		return fmt.Sprintf("Nothing in the graph matches %q.", term)
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Related to %q:\n", term)

	var documents []string
	for _, node := range subgraph.Nodes {
		if node.Kind == graph.KindDocument {
			documents = append(documents, node.Name)
			continue
		}
		fmt.Fprintf(&builder, "- %s (%s, mentioned %d times)\n", node.Name, node.Kind, node.Mentions)
	}
	if len(documents) > 0 {
		fmt.Fprintf(&builder, "Documents: %s\n", strings.Join(documents, "; "))
	}
	if subgraph.Truncated {
		builder.WriteString("The traversal hit its node limit, so this is a partial view.\n")
	}
	return strings.TrimSpace(builder.String())
}
