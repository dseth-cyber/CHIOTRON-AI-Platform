// Package graph is the relationship layer over the document corpus.
//
// Provider is the abstraction ARCHITECTURE-v1 section 6 calls for: GraphRAG runs
// on AI-owned tables now, and moving to Neo4j must be an adapter change rather
// than an orchestration change. Nothing above this package may reference
// PostgreSQL, and nothing in it may assume a particular store either.
package graph

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var ErrNotFound = errors.New("node not found")

// Node kinds. Documents are nodes too, which is what lets a traversal walk from
// an entity to the text that mentions it and on to a citation.
const (
	KindEntity   = "entity"
	KindAcronym  = "acronym"
	KindDocument = "document"
)

// Relations the extractor produces.
const (
	// RelationMentionedIn links an entity to a document.
	RelationMentionedIn = "mentioned_in"
	// RelationCoOccursWith links two entities seen in the same chunk. It records
	// that they appear together, not what the relationship means: naming the
	// relation needs a capable model, and inventing a label would be worse than
	// admitting the edge is untyped.
	RelationCoOccursWith = "co_occurs_with"
)

// Access is the ABAC predicate set, identical to the one retrieval uses. It is
// applied at every hop of a traversal.
type Access struct {
	CompanyID       string
	Department      string
	Classifications []string
}

type Node struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"`
	Name           string         `json:"name"`
	Normalised     string         `json:"normalisedName"`
	Classification string         `json:"classification"`
	CompanyID      string         `json:"companyId,omitempty"`
	Department     string         `json:"department,omitempty"`
	Properties     map[string]any `json:"properties,omitempty"`
	Mentions       int            `json:"mentionCount"`
	UpdatedAt      time.Time      `json:"updatedAt,omitempty"`
}

type Edge struct {
	SourceID       string `json:"sourceId"`
	TargetID       string `json:"targetId"`
	Relation       string `json:"relation"`
	Weight         int    `json:"weight"`
	Classification string `json:"classification"`
	CompanyID      string `json:"companyId,omitempty"`
	Department     string `json:"department,omitempty"`
}

// Mention is a source link: where a node was seen.
type Mention struct {
	NodeName     string
	DocumentID   string
	ChunkOrdinal int
	Occurrences  int
}

// Projection is what one document contributed, ready to be written in one go.
//
// Edge endpoints and mention node names hold *normalised names*, not identifiers:
// the extractor cannot know what a store calls its rows. Resolving them to
// identifiers is the provider's job, which is what keeps extraction free of any
// assumption about where the graph lives.
type Projection struct {
	Nodes    []Node
	Edges    []Edge
	Mentions []Mention
}

// Traversal bounds a walk. Both bounds exist because either alone is
// insufficient: depth without a node cap can still fan out to the whole graph.
type Traversal struct {
	Depth     int
	MaxNodes  int
	Relations []string
}

func NewTraversal(depth, maxNodes int, relations []string) (Traversal, error) {
	switch {
	case depth < 1:
		return Traversal{}, fmt.Errorf("traversal depth must be at least 1, got %d", depth)
	case maxNodes < 1:
		return Traversal{}, fmt.Errorf("traversal node cap must be at least 1, got %d", maxNodes)
	}
	return Traversal{Depth: depth, MaxNodes: maxNodes, Relations: relations}, nil
}

// Subgraph is a traversal result. Seeds are the nodes the query matched; Nodes
// includes them and everything reached.
type Subgraph struct {
	Seeds []Node `json:"seeds"`
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
	// Truncated reports that the node cap stopped the walk, so a caller can tell
	// "nothing more" from "too much".
	Truncated bool `json:"truncated"`
}

// DocumentIDs returns the documents reachable in the subgraph, which is how a
// traversal feeds retrieval: entities lead to text, text leads to citations.
func (s Subgraph) DocumentIDs() []string {
	var ids []string
	seen := make(map[string]bool)
	for _, node := range s.Nodes {
		if node.Kind != KindDocument {
			continue
		}
		id, ok := node.Properties["documentId"].(string)
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

type Stats struct {
	Nodes    int `json:"nodes"`
	Edges    int `json:"edges"`
	Mentions int `json:"mentions"`
}

// Provider is the only surface orchestration depends on.
//
// An implementation must satisfy the conformance suite in provider_conformance.go:
// that is what makes swapping in Neo4j a contained change rather than a rewrite
// with new behaviour.
type Provider interface {
	Name() string
	// Project writes one document's contribution. It must be idempotent:
	// re-ingesting a document is normal and must not multiply weights.
	Project(ctx context.Context, documentID string, projection Projection) error
	// Search finds seed nodes by name fragment, subject to access.
	Search(ctx context.Context, term string, access Access, limit int) ([]Node, error)
	// Neighbours walks out from seeds, filtering at every hop.
	Neighbours(ctx context.Context, seeds []string, traversal Traversal, access Access) (Subgraph, error)
	// Forget removes a document's contribution, for when it is withdrawn.
	Forget(ctx context.Context, documentID string) error
	Stats(ctx context.Context, access Access) (Stats, error)
}

// Normalise is the identity function for a node name. It has to be stable across
// providers, or the same entity would be two nodes after a migration.
func Normalise(name string) string {
	return strings.ToLower(strings.Join(strings.Fields(name), " "))
}

// sortNodes gives a traversal a stable result. Without it the same query returns
// the same subgraph in a different order each time, which makes a cited answer
// impossible to reproduce.
func sortNodes(nodes []Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Mentions != nodes[j].Mentions {
			return nodes[i].Mentions > nodes[j].Mentions
		}
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind < nodes[j].Kind
		}
		return nodes[i].Normalised < nodes[j].Normalised
	})
}
