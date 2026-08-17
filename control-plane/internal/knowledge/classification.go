// Package knowledge owns the document corpus: ingestion, chunking, embedding
// and permission-filtered retrieval.
//
// Retrieval applies identity, company, department and classification predicates
// before ranking, so content the caller may not see never reaches the model
// (ARCHITECTURE-v1 section 6).
package knowledge

import (
	"errors"
	"fmt"
	"strings"
)

var ErrUnknownClassification = errors.New("unknown classification")

// Policy is the ordered classification ladder. The order is configuration: a
// deployment may rename or extend the levels without code changes
// (ARCHITECTURE-v1 section 5).
type Policy struct {
	levels []string
	rank   map[string]int
}

// NewPolicy reads levels from least to most sensitive.
func NewPolicy(levels []string) (Policy, error) {
	if len(levels) == 0 {
		return Policy{}, fmt.Errorf("at least one classification level is required")
	}

	policy := Policy{levels: make([]string, 0, len(levels)), rank: make(map[string]int, len(levels))}
	for _, level := range levels {
		level = strings.ToLower(strings.TrimSpace(level))
		if level == "" {
			continue
		}
		if _, duplicate := policy.rank[level]; duplicate {
			return Policy{}, fmt.Errorf("classification %q is listed twice", level)
		}
		policy.rank[level] = len(policy.levels)
		policy.levels = append(policy.levels, level)
	}
	if len(policy.levels) == 0 {
		return Policy{}, fmt.Errorf("at least one classification level is required")
	}
	return policy, nil
}

func (p Policy) Levels() []string { return append([]string(nil), p.levels...) }

// Lowest is the least sensitive level, used as the default for a source that
// does not state one.
func (p Policy) Lowest() string { return p.levels[0] }

// Normalise validates a level and returns its canonical form.
func (p Policy) Normalise(level string) (string, error) {
	canonical := strings.ToLower(strings.TrimSpace(level))
	if _, known := p.rank[canonical]; !known {
		return "", fmt.Errorf("%w: %q (known levels: %s)", ErrUnknownClassification, level, strings.Join(p.levels, ", "))
	}
	return canonical, nil
}

// Readable returns every level a caller cleared to `ceiling` may read.
//
// The result is an explicit allow-list rather than a comparison, so the SQL
// predicate is a plain `= ANY(...)` that an index can serve.
func (p Policy) Readable(ceiling string) ([]string, error) {
	canonical, err := p.Normalise(ceiling)
	if err != nil {
		return nil, err
	}
	limit := p.rank[canonical]
	return append([]string(nil), p.levels[:limit+1]...), nil
}

// Allows reports whether a caller cleared to `ceiling` may read `level`. An
// unknown level is never readable: failing closed is the only safe reading of a
// classification the policy does not recognise.
func (p Policy) Allows(ceiling, level string) bool {
	ceilingRank, known := p.rank[strings.ToLower(strings.TrimSpace(ceiling))]
	if !known {
		return false
	}
	levelRank, known := p.rank[strings.ToLower(strings.TrimSpace(level))]
	if !known {
		return false
	}
	return levelRank <= ceilingRank
}
