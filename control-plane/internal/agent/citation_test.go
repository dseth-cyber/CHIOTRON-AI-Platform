package agent

import (
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

func hit(chunkID int64, documentID, title string, ordinal int, score float64) knowledge.Hit {
	return hitWithVector(chunkID, documentID, title, ordinal, score, score)
}

func hitWithVector(chunkID int64, documentID, title string, ordinal int, score, vector float64) knowledge.Hit {
	return knowledge.Hit{
		ChunkID: chunkID, DocumentID: documentID, DocumentTitle: title,
		Ordinal: ordinal, Content: title + " body", Score: score, VectorScore: vector,
	}
}

// Multi-round retrieval returns overlapping chunks by design. Offering the same
// passage twice would let the model cite two numbers for one source.
func TestCitationsDeduplicateChunks(t *testing.T) {
	citations := Citations([]knowledge.Hit{
		hit(1, "doc-a", "Runbook", 0, 0.03),
		hit(2, "doc-a", "Runbook", 1, 0.02),
		hit(1, "doc-a", "Runbook", 0, 0.03),
	})

	if len(citations) != 2 {
		t.Fatalf("got %d citations, want 2 distinct chunks", len(citations))
	}
	if citations[0].Index != 1 || citations[1].Index != 2 {
		t.Errorf("citations are not numbered from one: %+v", citations)
	}
}

// The numbering in the prompt has to match the citation list exactly, or an
// answer's references point at the wrong source.
func TestRenderContextNumberingMatchesCitations(t *testing.T) {
	hits := []knowledge.Hit{
		hit(1, "doc-a", "Alpha", 0, 0.03),
		hit(1, "doc-a", "Alpha", 0, 0.03),
		hit(2, "doc-b", "Beta", 4, 0.02),
	}
	citations := Citations(hits)
	rendered := RenderContext(hits, citations)

	if strings.Count(rendered, "[1]") != 1 || strings.Count(rendered, "[2]") != 1 {
		t.Fatalf("rendered context is not numbered once per citation:\n%s", rendered)
	}
	if strings.Contains(rendered, "[3]") {
		t.Errorf("rendered context numbered a duplicate passage:\n%s", rendered)
	}
	// Part numbers are one-based for a reader, while ordinals are zero-based.
	if !strings.Contains(rendered, "Beta (part 5)") {
		t.Errorf("rendered context does not name the passage position:\n%s", rendered)
	}
}

// A model can be handed passages and ignore them entirely. This is the check
// that separates a grounded answer from one that merely had context available.
func TestMarkUsedFindsReferencedCitations(t *testing.T) {
	citations := Citations([]knowledge.Hit{
		hit(1, "doc-a", "Alpha", 0, 0.03),
		hit(2, "doc-b", "Beta", 0, 0.02),
		hit(3, "doc-c", "Gamma", 0, 0.01),
	})

	marked, used := MarkUsed("Rotate the key [1], then revoke the old one [3].", citations)
	if len(used) != 2 || used[0] != 1 || used[1] != 3 {
		t.Fatalf("used = %v, want [1 3]", used)
	}
	if !marked[0].Used || marked[1].Used || !marked[2].Used {
		t.Errorf("marked flags = %v, %v, %v; want true, false, true",
			marked[0].Used, marked[1].Used, marked[2].Used)
	}
}

func TestMarkUsedReportsAnUngroundedAnswer(t *testing.T) {
	citations := Citations([]knowledge.Hit{hit(1, "doc-a", "Alpha", 0, 0.03)})

	marked, used := MarkUsed("Keys should be rotated regularly.", citations)
	if len(used) != 0 {
		t.Errorf("used = %v, want none for an answer with no markers", used)
	}
	if marked[0].Used {
		t.Error("a citation was marked used without being referenced")
	}
}

// A marker pointing at a passage that was never offered must not be counted as
// grounding.
func TestMarkUsedIgnoresOutOfRangeMarkers(t *testing.T) {
	citations := Citations([]knowledge.Hit{hit(1, "doc-a", "Alpha", 0, 0.03)})

	_, used := MarkUsed("According to [7] this is settled.", citations)
	if len(used) != 0 {
		t.Errorf("used = %v, want none for a marker with no citation", used)
	}
}

// Conflict detection is about competing sources, not contradictory statements:
// two different documents that are both genuinely about the question.
func TestConflictedDetectsCompetingDocuments(t *testing.T) {
	close := Citations([]knowledge.Hit{
		hitWithVector(1, "doc-a", "Alpha", 0, 0.0330, 0.80),
		hitWithVector(2, "doc-b", "Beta", 0, 0.0320, 0.76),
	})
	if !Conflicted(close, 0.15) {
		t.Error("two documents of comparable relevance were not flagged")
	}
}

// Fused scores compress by construction, so two documents can score within a
// hair of each other while being about entirely different subjects. Only the
// semantic score separates them.
func TestConflictedIgnoresCloseFusedScoresWithDistantMeaning(t *testing.T) {
	citations := Citations([]knowledge.Hit{
		hitWithVector(1, "doc-a", "Compute capacity", 0, 0.0328, 0.786),
		hitWithVector(2, "doc-b", "Key rotation runbook", 0, 0.0323, 0.349),
	})

	if Conflicted(citations, 0.15) {
		t.Error("documents with near-equal fused scores but unrelated meaning were flagged")
	}
}

// A keyword-only match carries no semantic signal to compare against.
func TestConflictedIgnoresKeywordOnlyMatches(t *testing.T) {
	citations := Citations([]knowledge.Hit{
		hitWithVector(1, "doc-a", "Alpha", 0, 0.0160, 0),
		hitWithVector(2, "doc-b", "Beta", 0, 0.0159, 0),
	})

	if Conflicted(citations, 0.15) {
		t.Error("keyword-only matches were flagged as competing answers")
	}
}

// Two passages from the same document are the same source, however close their
// scores.
func TestConflictedIgnoresSameDocument(t *testing.T) {
	sameDoc := Citations([]knowledge.Hit{
		hit(1, "doc-a", "Alpha", 0, 0.0330),
		hit(2, "doc-a", "Alpha", 1, 0.0329),
	})
	if Conflicted(sameDoc, 0.15) {
		t.Error("two passages from one document were flagged as conflicting")
	}
}

func TestConflictedHandlesThinEvidence(t *testing.T) {
	if Conflicted(nil, 0.15) {
		t.Error("no citations were flagged as conflicting")
	}
	single := Citations([]knowledge.Hit{hit(1, "doc-a", "Alpha", 0, 0.03)})
	if Conflicted(single, 0.15) {
		t.Error("a single citation was flagged as conflicting")
	}
	// A zero best score means nothing was really retrieved; there is no ratio to
	// compare and dividing by it would be meaningless.
	zero := Citations([]knowledge.Hit{
		hit(1, "doc-a", "Alpha", 0, 0),
		hit(2, "doc-b", "Beta", 0, 0),
	})
	if Conflicted(zero, 0.15) {
		t.Error("zero-scoring citations were flagged as conflicting")
	}
}
