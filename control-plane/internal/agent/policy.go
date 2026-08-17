// Package agent orchestrates a grounded answer: plan, retrieve, optionally call
// tools, then synthesise with citations.
//
// The planner is deterministic policy rather than a model deciding its own next
// move (ARCHITECTURE-v1 section 6 calls for "intent and planner policy"). A rule
// can be read, tested and audited; asking a 0.5B model to plan its own retrieval
// would make every answer's shape unexplainable.
package agent

import (
	"fmt"
	"strings"
)

// Retrieval modes an assistant may be configured with.
const (
	// RetrievalOff answers from the model alone.
	RetrievalOff = "off"
	// RetrievalAuto retrieves unless the question is too short to be a question.
	RetrievalAuto = "auto"
	// RetrievalAlways grounds every answer, and says so when it cannot.
	RetrievalAlways = "always"
)

// Policy is the planner's configuration.
type Policy struct {
	// MaxSteps bounds the whole run, retrieval rounds and tool calls together.
	MaxSteps int
	// TopK is how many passages one retrieval round returns.
	TopK int
	// MinScore is the fused score a round must beat to be treated as useful.
	// Below it, the planner either expands the query or gives up on grounding.
	MinScore float64
	// ConflictMargin is how close the top two hits must be, relative to the
	// best, before differing documents are treated as disagreeing.
	ConflictMargin float64
}

func NewPolicy(maxSteps, topK int, minScore, conflictMargin float64) (Policy, error) {
	switch {
	case maxSteps < 1:
		return Policy{}, fmt.Errorf("agent max steps must be at least 1, got %d", maxSteps)
	case topK < 1:
		return Policy{}, fmt.Errorf("agent top-k must be at least 1, got %d", topK)
	case minScore < 0:
		return Policy{}, fmt.Errorf("agent minimum score cannot be negative, got %v", minScore)
	case conflictMargin < 0 || conflictMargin > 1:
		return Policy{}, fmt.Errorf("agent conflict margin must be between 0 and 1, got %v", conflictMargin)
	}
	return Policy{MaxSteps: maxSteps, TopK: topK, MinScore: minScore, ConflictMargin: conflictMargin}, nil
}

// NormaliseMode validates a retrieval mode.
func NormaliseMode(mode string) (string, error) {
	switch normalised := strings.ToLower(strings.TrimSpace(mode)); normalised {
	case RetrievalOff, RetrievalAuto, RetrievalAlways:
		return normalised, nil
	case "":
		return RetrievalAuto, nil
	default:
		return "", fmt.Errorf("retrieval mode %q is not off, auto or always", mode)
	}
}

// minimumWordsToRetrieve is the intent rule for auto mode: a greeting or an
// acknowledgement is not a question about the corpus, and searching for it wastes
// an embedding call and pollutes the prompt.
const minimumWordsToRetrieve = 3

// ShouldRetrieve applies the intent rule.
//
// It is deliberately crude. The expensive judgement is left to the score
// threshold after retrieval, which is evidence about this corpus rather than a
// guess about this wording.
func ShouldRetrieve(mode, question string) bool {
	switch mode {
	case RetrievalOff:
		return false
	case RetrievalAlways:
		return true
	default:
		return len(strings.Fields(question)) >= minimumWordsToRetrieve
	}
}

// ExpandQuery builds a follow-up query from the best passage so far.
//
// Anchoring on the strongest document's title pulls in its neighbouring chunks,
// which is where an answer split across a chunk boundary usually hides. With no
// candidate to anchor on there is nothing to expand from, so the caller stops.
func ExpandQuery(question, bestDocumentTitle string) (string, bool) {
	title := strings.TrimSpace(bestDocumentTitle)
	if title == "" {
		return "", false
	}
	// A title already contained in the question adds nothing.
	if strings.Contains(strings.ToLower(question), strings.ToLower(title)) {
		return "", false
	}
	return title + " " + question, true
}
