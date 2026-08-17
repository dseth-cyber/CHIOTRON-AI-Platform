package knowledge

import (
	"errors"
	"strings"
	"testing"
)

func testPolicy(t *testing.T) Policy {
	t.Helper()
	policy, err := NewPolicy([]string{"public", "internal", "confidential", "restricted"})
	if err != nil {
		t.Fatalf("NewPolicy() returned error: %v", err)
	}
	return policy
}

func TestNewPolicyRejectsBadLadders(t *testing.T) {
	cases := map[string][]string{
		"empty":     {},
		"all blank": {"", "   "},
		"duplicate": {"public", "internal", "public"},
	}
	for name, levels := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewPolicy(levels); err == nil {
				t.Fatalf("NewPolicy(%v) succeeded, want error", levels)
			}
		})
	}
}

func TestNewPolicyNormalisesInput(t *testing.T) {
	policy, err := NewPolicy([]string{" Public ", "INTERNAL"})
	if err != nil {
		t.Fatalf("NewPolicy() returned error: %v", err)
	}
	if got := policy.Levels(); len(got) != 2 || got[0] != "public" || got[1] != "internal" {
		t.Errorf("Levels() = %v, want lowercased and trimmed", got)
	}
	if policy.Lowest() != "public" {
		t.Errorf("Lowest() = %q, want the first level", policy.Lowest())
	}
}

// A ceiling grants everything at or below it, as an explicit list an index can
// serve rather than a comparison.
func TestReadable(t *testing.T) {
	policy := testPolicy(t)

	cases := map[string][]string{
		"public":       {"public"},
		"internal":     {"public", "internal"},
		"confidential": {"public", "internal", "confidential"},
		"restricted":   {"public", "internal", "confidential", "restricted"},
	}
	for ceiling, want := range cases {
		t.Run(ceiling, func(t *testing.T) {
			got, err := policy.Readable(ceiling)
			if err != nil {
				t.Fatalf("Readable(%q) returned error: %v", ceiling, err)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("Readable(%q) = %v, want %v", ceiling, got, want)
			}
		})
	}
}

func TestReadableRejectsUnknownCeiling(t *testing.T) {
	policy := testPolicy(t)

	_, err := policy.Readable("top-secret")
	if !errors.Is(err, ErrUnknownClassification) {
		t.Fatalf("error = %v, want ErrUnknownClassification", err)
	}
	// The message has to name the valid levels or an operator cannot fix it.
	if !strings.Contains(err.Error(), "confidential") {
		t.Errorf("error %q does not list the known levels", err)
	}
}

func TestAllows(t *testing.T) {
	policy := testPolicy(t)

	if !policy.Allows("confidential", "internal") {
		t.Error("a confidential clearance was denied internal content")
	}
	if !policy.Allows("internal", "internal") {
		t.Error("a clearance was denied its own level")
	}
	if policy.Allows("internal", "confidential") {
		t.Error("an internal clearance was granted confidential content")
	}
}

// A classification the policy does not recognise must never be readable.
// Failing closed is the only safe reading of an unknown label.
func TestAllowsFailsClosedOnUnknownLevels(t *testing.T) {
	policy := testPolicy(t)

	if policy.Allows("restricted", "cosmic") {
		t.Error("an unknown content classification was allowed")
	}
	if policy.Allows("cosmic", "public") {
		t.Error("an unknown clearance was allowed to read")
	}
	if policy.Allows("", "public") {
		t.Error("an empty clearance was allowed to read")
	}
}

// A zero Policy is what a Deps assembled without config.Load carries, and it
// must deny rather than permit.
func TestZeroPolicyDeniesEverything(t *testing.T) {
	var policy Policy

	if policy.Allows("restricted", "public") {
		t.Error("the zero policy allowed a read")
	}
}
