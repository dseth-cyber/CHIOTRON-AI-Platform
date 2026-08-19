// Package agent: Synthetic Benchmark and Evaluation Suite for Agentic RAG.
package agent

import (
	"context"
	"math"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

// TestCase defines a single synthetic RAG evaluation prompt with expected targets.
type TestCase struct {
	ID                     string           `json:"id"`
	Query                  string           `json:"query"`
	Access                 knowledge.Access `json:"access,omitempty"`
	GroundTruthDocumentIDs []string         `json:"groundTruthDocumentIds"`
	ExpectedConflict       bool             `json:"expectedConflict"`
}

// SearchParams encapsulates query parameters passed to the search function during evaluation.
type SearchParams struct {
	Query  string
	Access knowledge.Access
	Limit  int
}

// EvalResult records metrics for a single test case execution.
type EvalResult struct {
	TestCaseID       string  `json:"testCaseId"`
	RetrievedCount   int     `json:"retrievedCount"`
	Precision        float64 `json:"precision"`
	Recall           float64 `json:"recall"`
	ReciprocalRank   float64 `json:"reciprocalRank"`
	ConflictDetected bool    `json:"conflictDetected"`
	ConflictCorrect  bool    `json:"conflictCorrect"`
	GroundingScore   float64 `json:"groundingScore"`
}

// EvalReport aggregates evaluation results across the benchmark suite.
type EvalReport struct {
	TotalTests         int          `json:"totalTests"`
	MeanPrecision      float64      `json:"meanPrecision"`
	MeanRecall         float64      `json:"meanRecall"`
	MRR                float64      `json:"mrr"`
	MeanGroundingScore float64      `json:"meanGroundingScore"`
	ConflictAccuracy   float64      `json:"conflictAccuracy"`
	Results            []EvalResult `json:"results"`
}

// EvaluateCase evaluates retrieval hits and an answer against ground truth.
func EvaluateCase(test TestCase, hits []knowledge.Hit, answer string, conflictMargin float64) EvalResult {
	citations := Citations(hits)
	marked, used := MarkUsed(answer, citations)
	isConflicted := Conflicted(citations, conflictMargin)

	// Ground truth match map
	gtMap := make(map[string]bool, len(test.GroundTruthDocumentIDs))
	for _, id := range test.GroundTruthDocumentIDs {
		gtMap[strings.TrimSpace(id)] = true
	}

	relevantRetrieved := 0
	firstRelevantRank := 0
	for i, hit := range hits {
		if gtMap[hit.DocumentID] {
			relevantRetrieved++
			if firstRelevantRank == 0 {
				firstRelevantRank = i + 1
			}
		}
	}

	precision := 0.0
	if len(hits) > 0 {
		precision = float64(relevantRetrieved) / float64(len(hits))
	}

	recall := 0.0
	if len(test.GroundTruthDocumentIDs) > 0 {
		recall = float64(relevantRetrieved) / float64(len(test.GroundTruthDocumentIDs))
	} else if len(hits) == 0 {
		recall = 1.0
	}

	rr := 0.0
	if firstRelevantRank > 0 {
		rr = 1.0 / float64(firstRelevantRank)
	}

	groundingScore := 0.0
	if len(used) > 0 && len(marked) > 0 {
		validCited := 0
		for _, idx := range used {
			if idx >= 1 && idx <= len(marked) {
				validCited++
			}
		}
		groundingScore = float64(validCited) / float64(len(used))
	} else if len(hits) == 0 {
		groundingScore = 1.0
	}

	conflictCorrect := (isConflicted == test.ExpectedConflict)

	return EvalResult{
		TestCaseID:       test.ID,
		RetrievedCount:   len(hits),
		Precision:        precision,
		Recall:           math.Min(recall, 1.0),
		ReciprocalRank:   rr,
		ConflictDetected: isConflicted,
		ConflictCorrect:  conflictCorrect,
		GroundingScore:   groundingScore,
	}
}

// DefaultSyntheticEvalSet provides a standardized benchmark suite for testing Agentic RAG.
func DefaultSyntheticEvalSet() []TestCase {
	return []TestCase{
		{
			ID:    "eval-01-security-policy",
			Query: "What is the company password rotation and MFA policy?",
			Access: knowledge.Access{
				CompanyID:       "acme",
				Classifications: []string{"public", "internal"},
			},
			GroundTruthDocumentIDs: []string{"doc-sec-01", "doc-sec-02"},
			ExpectedConflict:       false,
		},
		{
			ID:    "eval-02-competing-expense-rules",
			Query: "What is the maximum allowed travel hotel expense per night?",
			Access: knowledge.Access{
				CompanyID:       "acme",
				Classifications: []string{"public", "internal"},
			},
			GroundTruthDocumentIDs: []string{"doc-policy-2024", "doc-policy-2025"},
			ExpectedConflict:       true,
		},
		{
			ID:    "eval-03-cross-tenant-isolation",
			Query: "Show me financial records for Globex Q3 earnings",
			Access: knowledge.Access{
				CompanyID:       "acme",
				Classifications: []string{"public", "internal", "confidential"},
			},
			GroundTruthDocumentIDs: []string{},
			ExpectedConflict:       false,
		},
		{
			ID:    "eval-04-technical-architecture",
			Query: "How does the control plane isolate VM4 from VM5 compute inference?",
			Access: knowledge.Access{
				CompanyID:       "_platform",
				Classifications: []string{"public", "internal"},
			},
			GroundTruthDocumentIDs: []string{"doc-arch-v1"},
			ExpectedConflict:       false,
		},
		{
			ID:    "eval-05-procurement-workflow",
			Query: "What are the approval thresholds for purchase orders above 50,000 THB?",
			Access: knowledge.Access{
				CompanyID:       "acme",
				Department:      "procurement",
				Classifications: []string{"public", "internal"},
			},
			GroundTruthDocumentIDs: []string{"doc-procure-01"},
			ExpectedConflict:       false,
		},
	}
}

// RunEvaluation executes a test suite and compiles the benchmark report.
func RunEvaluation(
	ctx context.Context,
	suite []TestCase,
	searchFn func(ctx context.Context, p SearchParams) ([]knowledge.Hit, error),
	answerFn func(ctx context.Context, query string, hits []knowledge.Hit) (string, error),
	conflictMargin float64,
) (EvalReport, error) {
	results := make([]EvalResult, 0, len(suite))
	var sumPrecision, sumRecall, sumRR, sumGrounding float64
	correctConflicts := 0

	for _, tc := range suite {
		if err := ctx.Err(); err != nil {
			return EvalReport{}, err
		}

		hits, err := searchFn(ctx, SearchParams{
			Query:  tc.Query,
			Access: tc.Access,
			Limit:  5,
		})
		if err != nil {
			return EvalReport{}, err
		}

		answer := ""
		if answerFn != nil {
			answer, err = answerFn(ctx, tc.Query, hits)
			if err != nil {
				return EvalReport{}, err
			}
		}

		res := EvaluateCase(tc, hits, answer, conflictMargin)
		results = append(results, res)

		sumPrecision += res.Precision
		sumRecall += res.Recall
		sumRR += res.ReciprocalRank
		sumGrounding += res.GroundingScore
		if res.ConflictCorrect {
			correctConflicts++
		}
	}

	count := float64(len(suite))
	if count == 0 {
		return EvalReport{}, nil
	}

	return EvalReport{
		TotalTests:         len(suite),
		MeanPrecision:      sumPrecision / count,
		MeanRecall:         sumRecall / count,
		MRR:                sumRR / count,
		MeanGroundingScore: sumGrounding / count,
		ConflictAccuracy:   float64(correctConflicts) / count,
		Results:            results,
	}, nil
}
