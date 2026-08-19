package graph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNeo4jProviderOperations(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[],"errors":[]}`))
	}))
	defer ts.Close()

	neo, err := NewNeo4j(Neo4jConfig{
		Endpoint: ts.URL,
		Database: "neo4j",
		Username: "neo4j",
		Password: "password",
	})
	if err != nil {
		t.Fatalf("NewNeo4j error: %v", err)
	}

	if neo.Name() != "neo4j" {
		t.Errorf("Name() = %q, want 'neo4j'", neo.Name())
	}

	ctx := context.Background()
	access := Access{
		CompanyID:       "acme",
		Department:      "engineering",
		Classifications: []string{"public", "internal"},
	}

	// 1. Project
	proj := Projection{
		Nodes: []Node{
			{Name: "Control Plane", Kind: KindEntity, Classification: "internal", CompanyID: "acme", Department: "engineering"},
			{Name: "Compute Plane", Kind: KindEntity, Classification: "internal", CompanyID: "acme", Department: "engineering"},
		},
		Edges: []Edge{
			{SourceID: "acme:entity:control plane", TargetID: "acme:entity:compute plane", Relation: RelationCoOccursWith, Weight: 1, Classification: "internal", CompanyID: "acme", Department: "engineering"},
		},
		Mentions: []Mention{
			{NodeName: "Control Plane", DocumentID: "doc-101", ChunkOrdinal: 0, Occurrences: 3},
		},
	}

	if err := neo.Project(ctx, "doc-101", proj); err != nil {
		t.Fatalf("Project() error: %v", err)
	}

	// 2. Search
	nodes, err := neo.Search(ctx, "control", access, 5)
	if err != nil {
		t.Fatalf("Search() error: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "Control Plane" {
		t.Errorf("Unexpected Search results: %+v", nodes)
	}

	// 3. Neighbours Traversal
	trav, _ := NewTraversal(2, 10, nil)
	subgraph, err := neo.Neighbours(ctx, []string{"acme:entity:control plane"}, trav, access)
	if err != nil {
		t.Fatalf("Neighbours() error: %v", err)
	}
	if len(subgraph.Seeds) != 1 {
		t.Errorf("Expected 1 seed, got %d", len(subgraph.Seeds))
	}
	if len(subgraph.Nodes) != 2 {
		t.Errorf("Expected 2 nodes reached, got %d", len(subgraph.Nodes))
	}
	if len(subgraph.Edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(subgraph.Edges))
	}

	// 4. Stats
	stats, err := neo.Stats(ctx, access)
	if err != nil {
		t.Fatalf("Stats() error: %v", err)
	}
	if stats.Nodes != 2 || stats.Edges != 1 || stats.Mentions != 1 {
		t.Errorf("Unexpected stats: %+v", stats)
	}

	// 5. Cypher execute
	_, err = neo.CypherExecute(ctx, "MATCH (n) RETURN count(n)", nil)
	if err != nil {
		t.Fatalf("CypherExecute() error: %v", err)
	}

	// 6. Forget
	if err := neo.Forget(ctx, "doc-101"); err != nil {
		t.Fatalf("Forget() error: %v", err)
	}
}
