package graph

import (
	"strings"
	"testing"
)

func testSource() Source {
	return Source{
		DocumentID: "dddddddd-dddd-dddd-dddd-dddddddddddd",
		Title:      "Compute capacity note", Classification: "internal",
		CompanyID: "acme", Department: "platform",
	}
}

func nodeNames(projection Projection) map[string]string {
	names := make(map[string]string, len(projection.Nodes))
	for _, node := range projection.Nodes {
		names[node.Normalised] = node.Kind
	}
	return names
}

func TestExtractMakesTheDocumentANode(t *testing.T) {
	projection := Extract(testSource(), []string{"The Control Plane decides."})

	document := projection.Nodes[0]
	if document.Kind != KindDocument {
		t.Fatalf("first node kind = %q, want %q", document.Kind, KindDocument)
	}
	// The document id has to travel on the node, because that is how a traversal
	// gets from an entity back to citable text.
	if document.Properties["documentId"] != testSource().DocumentID {
		t.Errorf("document node properties = %v, want the document id", document.Properties)
	}
}

func TestExtractFindsEntitiesAndAcronyms(t *testing.T) {
	projection := Extract(testSource(), []string{
		"The Control Plane runs on VM4. The Compute Plane runs on VM5 with a Quadro P620.",
	})
	names := nodeNames(projection)

	for _, want := range []string{"control plane", "compute plane"} {
		if names[want] != KindEntity {
			t.Errorf("%q was not extracted as an entity: %v", want, names)
		}
	}
	// VM4 and P620 are exactly the identifiers this corpus is full of.
	for _, want := range []string{"vm4", "vm5", "p620"} {
		if names[want] != KindAcronym {
			t.Errorf("%q was not extracted as an acronym: %v", want, names)
		}
	}
}

// A sentence's first word is capitalised by grammar, so on its own it is no
// evidence of a name.
func TestExtractIgnoresSentenceInitialCapitals(t *testing.T) {
	projection := Extract(testSource(), []string{"The service restarts. Documents are stored."})
	names := nodeNames(projection)

	for _, unwanted := range []string{"the", "documents", "the service"} {
		if _, present := names[unwanted]; present {
			t.Errorf("%q was extracted from a sentence-initial capital: %v", unwanted, names)
		}
	}
}

// Casing variants have to collapse, or the same entity becomes two nodes and the
// graph splits in half.
func TestExtractCollapsesCasingVariants(t *testing.T) {
	projection := Extract(testSource(), []string{
		"We deploy the Control Plane.",
		"We restart the CONTROL PLANE later.",
	})

	count := 0
	for _, node := range projection.Nodes {
		if node.Normalised == "control plane" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("got %d nodes for one entity, want 1", count)
	}
}

// Adjacent title-case words are one name, not several.
func TestExtractJoinsMultiWordNames(t *testing.T) {
	projection := Extract(testSource(), []string{"We use the Enterprise AI Platform daily."})
	names := nodeNames(projection)

	if _, present := names["enterprise ai platform"]; !present {
		t.Errorf("the multi-word name was not joined: %v", names)
	}
}

// A trailing acronym is a separate thing, not part of the name before it: the
// Control Plane merely runs on VM4.
func TestExtractDoesNotAbsorbTrailingAcronyms(t *testing.T) {
	projection := Extract(testSource(), []string{"We deploy the Control Plane VM4 tonight."})
	names := nodeNames(projection)

	if _, present := names["control plane"]; !present {
		t.Errorf("the entity was not extracted on its own: %v", names)
	}
	if _, fused := names["control plane vm4"]; fused {
		t.Errorf("a trailing acronym was fused into the name: %v", names)
	}
	if names["vm4"] != KindAcronym {
		t.Errorf("the released acronym was not extracted separately: %v", names)
	}
}

func TestExtractLinksEntitiesToTheDocument(t *testing.T) {
	projection := Extract(testSource(), []string{"The Control Plane runs on VM4."})

	var mentioned int
	for _, edge := range projection.Edges {
		if edge.Relation == RelationMentionedIn && edge.TargetID == Normalise(testSource().Title) {
			mentioned++
		}
	}
	if mentioned == 0 {
		t.Errorf("no entity was linked to the document: %+v", projection.Edges)
	}
}

// Co-occurrence is within a chunk, not a document: two entities in one passage are
// plausibly related, two in a fifty-page document are not.
func TestExtractCoOccurrenceIsPerChunk(t *testing.T) {
	together := Extract(testSource(), []string{"The Control Plane calls the Compute Plane."})
	apart := Extract(testSource(), []string{
		"The Control Plane decides.",
		"The Compute Plane infers.",
	})

	if countRelation(together, RelationCoOccursWith) == 0 {
		t.Error("entities in one chunk were not linked")
	}
	if countRelation(apart, RelationCoOccursWith) != 0 {
		t.Error("entities in separate chunks were linked as co-occurring")
	}
}

// Edge weight is what traversal ranks by, so it has to count observations rather
// than exist as a flag.
func TestExtractWeighsRepeatedObservations(t *testing.T) {
	projection := Extract(testSource(), []string{
		"The Control Plane calls the Compute Plane.",
		"Again the Control Plane calls the Compute Plane.",
	})

	for _, edge := range projection.Edges {
		if edge.Relation == RelationCoOccursWith {
			if edge.Weight != 2 {
				t.Errorf("co-occurrence weight = %d, want 2 observations", edge.Weight)
			}
			return
		}
	}
	t.Error("no co-occurrence edge was produced")
}

// Every node and edge carries the document's ACL triple, so a traversal can
// filter without joining back to the document.
func TestExtractCopiesTheAccessTriple(t *testing.T) {
	projection := Extract(testSource(), []string{"The Control Plane runs on VM4."})

	for _, node := range projection.Nodes {
		if node.Classification != "internal" || node.CompanyID != "acme" || node.Department != "platform" {
			t.Fatalf("node %q did not inherit the document ACL: %+v", node.Normalised, node)
		}
	}
	for _, edge := range projection.Edges {
		if edge.Classification != "internal" || edge.CompanyID != "acme" {
			t.Fatalf("edge %s did not inherit the document ACL: %+v", edge.Relation, edge)
		}
	}
}

// Mentions are provenance: a relationship with no source link cannot be cited.
func TestExtractRecordsMentionOrdinals(t *testing.T) {
	projection := Extract(testSource(), []string{
		"The Control Plane decides.",
		"The Control Plane also audits.",
	})

	ordinals := map[int]bool{}
	for _, mention := range projection.Mentions {
		if mention.NodeName == "control plane" {
			ordinals[mention.ChunkOrdinal] = true
		}
	}
	if !ordinals[0] || !ordinals[1] {
		t.Errorf("mentions did not record both chunks: %+v", projection.Mentions)
	}
}

// The same document must always produce the same graph, or an edge weight means
// nothing and a traversal cannot be reproduced.
func TestExtractIsDeterministic(t *testing.T) {
	chunks := []string{
		"The Control Plane runs on VM4 and calls the Compute Plane on VM5.",
		"The Knowledge Platform stores documents in PostgreSQL.",
	}
	first := Extract(testSource(), chunks)
	second := Extract(testSource(), chunks)

	if len(first.Nodes) != len(second.Nodes) || len(first.Edges) != len(second.Edges) {
		t.Fatalf("projections differ in size: %d/%d nodes, %d/%d edges",
			len(first.Nodes), len(second.Nodes), len(first.Edges), len(second.Edges))
	}
	for i := range first.Nodes {
		if first.Nodes[i].Normalised != second.Nodes[i].Normalised {
			t.Fatalf("node %d differs between runs: %q and %q", i,
				first.Nodes[i].Normalised, second.Nodes[i].Normalised)
		}
	}
	for i := range first.Edges {
		if first.Edges[i].SourceID != second.Edges[i].SourceID ||
			first.Edges[i].TargetID != second.Edges[i].TargetID {
			t.Fatalf("edge %d differs between runs", i)
		}
	}
}

// Without a cap a long passage joins every entity to every other, which is a
// graph with no information in it.
func TestExtractCapsEntitiesPerChunk(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("Start here.")
	for i := range 40 {
		builder.WriteString(" We saw Alpha")
		builder.WriteByte(byte('A' + i%26))
		builder.WriteString(" today.")
	}

	projection := Extract(testSource(), []string{builder.String()})
	// The document node is always present, so the entity budget is the rest.
	if entities := len(projection.Nodes) - 1; entities > maxEntitiesPerChunk {
		t.Errorf("extracted %d entities, want at most %d", entities, maxEntitiesPerChunk)
	}
}

func TestNormaliseCollapsesWhitespaceAndCase(t *testing.T) {
	if got := Normalise("  Control   PLANE \n"); got != "control plane" {
		t.Errorf("Normalise() = %q, want %q", got, "control plane")
	}
}

func TestNewTraversalRejectsUnboundedWalks(t *testing.T) {
	if _, err := NewTraversal(0, 10, nil); err == nil {
		t.Error("NewTraversal() accepted zero depth")
	}
	if _, err := NewTraversal(2, 0, nil); err == nil {
		t.Error("NewTraversal() accepted a zero node cap")
	}
}

func TestSubgraphDocumentIDs(t *testing.T) {
	subgraph := Subgraph{Nodes: []Node{
		{Kind: KindEntity, Name: "Control Plane"},
		{Kind: KindDocument, Properties: map[string]any{"documentId": "doc-1"}},
		{Kind: KindDocument, Properties: map[string]any{"documentId": "doc-1"}},
		{Kind: KindDocument, Properties: map[string]any{"documentId": "doc-2"}},
	}}

	ids := subgraph.DocumentIDs()
	if len(ids) != 2 {
		t.Fatalf("DocumentIDs() = %v, want two distinct documents", ids)
	}
}

func countRelation(projection Projection, relation string) int {
	count := 0
	for _, edge := range projection.Edges {
		if edge.Relation == relation {
			count++
		}
	}
	return count
}
