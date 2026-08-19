package agent

import (
	"context"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

func TestEvaluateCasePrecisionAndRecall(t *testing.T) {
	tc := TestCase{
		ID:                     "test-1",
		Query:                  "password policy",
		GroundTruthDocumentIDs: []string{"doc-1", "doc-2"},
		ExpectedConflict:       false,
	}

	hits := []knowledge.Hit{
		{ChunkID: 101, DocumentID: "doc-1", DocumentTitle: "Security Policy", Ordinal: 0, Score: 0.9, VectorScore: 0.85, Content: "Use strong passwords."},
		{ChunkID: 102, DocumentID: "doc-3", DocumentTitle: "Unrelated Guide", Ordinal: 0, Score: 0.5, VectorScore: 0.40, Content: "Lunch menu."},
	}

	answer := "According to [1], use strong passwords."
	result := EvaluateCase(tc, hits, answer, 0.05)

	// 1 relevant out of 2 retrieved -> Precision = 0.5
	if result.Precision != 0.5 {
		t.Errorf("Precision = %f, want 0.5", result.Precision)
	}

	// 1 relevant out of 2 ground truths -> Recall = 0.5
	if result.Recall != 0.5 {
		t.Errorf("Recall = %f, want 0.5", result.Recall)
	}

	// First relevant is at rank 1 -> MRR = 1.0
	if result.ReciprocalRank != 1.0 {
		t.Errorf("ReciprocalRank = %f, want 1.0", result.ReciprocalRank)
	}

	// Cited [1] out of [1, 2] -> GroundingScore = 1.0
	if result.GroundingScore != 1.0 {
		t.Errorf("GroundingScore = %f, want 1.0", result.GroundingScore)
	}

	if result.ConflictDetected {
		t.Errorf("ConflictDetected = true, want false")
	}
}

func TestEvaluateCaseConflictDetection(t *testing.T) {
	tc := TestCase{
		ID:                     "test-conflict",
		Query:                  "hotel budget",
		GroundTruthDocumentIDs: []string{"doc-2024", "doc-2025"},
		ExpectedConflict:       true,
	}

	hits := []knowledge.Hit{
		{ChunkID: 201, DocumentID: "doc-2024", DocumentTitle: "Policy 2024", Ordinal: 0, Score: 0.92, VectorScore: 0.89, Content: "Hotel cap is 2000 THB."},
		{ChunkID: 202, DocumentID: "doc-2025", DocumentTitle: "Policy 2025", Ordinal: 0, Score: 0.91, VectorScore: 0.88, Content: "Hotel cap is 2500 THB."},
	}

	answer := "Sources [1] and [2] list different amounts."
	result := EvaluateCase(tc, hits, answer, 0.05)

	if !result.ConflictDetected {
		t.Errorf("ConflictDetected = false, want true")
	}
	if !result.ConflictCorrect {
		t.Errorf("ConflictCorrect = false, want true")
	}
}

func TestRunEvaluationBenchmarkSuite(t *testing.T) {
	suite := DefaultSyntheticEvalSet()
	if len(suite) == 0 {
		t.Fatal("DefaultSyntheticEvalSet is empty")
	}

	mockSearch := func(ctx context.Context, p SearchParams) ([]knowledge.Hit, error) {
		switch p.Query {
		case "What is the company password rotation and MFA policy?":
			return []knowledge.Hit{
				{ChunkID: 1, DocumentID: "doc-sec-01", Score: 0.9, VectorScore: 0.85},
				{ChunkID: 2, DocumentID: "doc-sec-02", Score: 0.8, VectorScore: 0.75},
			}, nil
		case "What is the maximum allowed travel hotel expense per night?":
			return []knowledge.Hit{
				{ChunkID: 3, DocumentID: "doc-policy-2024", Score: 0.95, VectorScore: 0.90},
				{ChunkID: 4, DocumentID: "doc-policy-2025", Score: 0.94, VectorScore: 0.89},
			}, nil
		default:
			return []knowledge.Hit{}, nil
		}
	}

	mockAnswer := func(ctx context.Context, q string, hits []knowledge.Hit) (string, error) {
		if len(hits) > 0 {
			return "Answer based on [1]", nil
		}
		return "No data found.", nil
	}

	report, err := RunEvaluation(context.Background(), suite, mockSearch, mockAnswer, 0.05)
	if err != nil {
		t.Fatalf("RunEvaluation failed: %v", err)
	}

	if report.TotalTests != len(suite) {
		t.Errorf("TotalTests = %d, want %d", report.TotalTests, len(suite))
	}
	if report.MRR <= 0 {
		t.Errorf("MRR = %f, want > 0", report.MRR)
	}
}
