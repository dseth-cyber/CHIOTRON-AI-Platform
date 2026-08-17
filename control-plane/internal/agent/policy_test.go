package agent

import "testing"

func TestNewPolicyRejectsUnworkableValues(t *testing.T) {
	cases := map[string]struct {
		steps, topK   int
		score, margin float64
	}{
		"no steps":       {0, 5, 0.01, 0.1},
		"no candidates":  {3, 0, 0.01, 0.1},
		"negative score": {3, 5, -1, 0.1},
		"margin over 1":  {3, 5, 0.01, 1.5},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewPolicy(tc.steps, tc.topK, tc.score, tc.margin); err == nil {
				t.Fatal("NewPolicy() succeeded, want error")
			}
		})
	}
}

func TestNormaliseMode(t *testing.T) {
	for _, input := range []string{"off", "OFF", " auto ", "always"} {
		if _, err := NormaliseMode(input); err != nil {
			t.Errorf("NormaliseMode(%q) returned error: %v", input, err)
		}
	}
	// An empty mode is the platform default rather than an error, so an assistant
	// created before the column existed still works.
	if mode, err := NormaliseMode(""); err != nil || mode != RetrievalAuto {
		t.Errorf("NormaliseMode(\"\") = %q, %v, want auto and no error", mode, err)
	}
	if _, err := NormaliseMode("sometimes"); err == nil {
		t.Error("NormaliseMode() accepted an unknown mode")
	}
}

// The intent rule is deliberately crude: it only avoids searching for things
// that are plainly not questions about the corpus.
func TestShouldRetrieve(t *testing.T) {
	cases := map[string]struct {
		mode, question string
		want           bool
	}{
		"off never retrieves":         {RetrievalOff, "what is the rotation procedure", false},
		"always retrieves a greeting": {RetrievalAlways, "hi", true},
		"auto retrieves a question":   {RetrievalAuto, "what is the rotation procedure", true},
		"auto skips a greeting":       {RetrievalAuto, "hi", false},
		"auto skips two words":        {RetrievalAuto, "thanks again", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ShouldRetrieve(tc.mode, tc.question); got != tc.want {
				t.Errorf("ShouldRetrieve(%q, %q) = %v, want %v", tc.mode, tc.question, got, tc.want)
			}
		})
	}
}

func TestExpandQueryAnchorsOnTheBestDocument(t *testing.T) {
	expanded, ok := ExpandQuery("how do I rotate a key", "Key rotation runbook")
	if !ok {
		t.Fatal("ExpandQuery() refused to expand with a candidate available")
	}
	if expanded != "Key rotation runbook how do I rotate a key" {
		t.Errorf("expanded = %q, want the title prefixed", expanded)
	}
}

// With nothing retrieved there is nothing to anchor on, and repeating the same
// search would return the same passages.
func TestExpandQueryRefusesWithoutACandidate(t *testing.T) {
	if _, ok := ExpandQuery("anything", ""); ok {
		t.Error("ExpandQuery() expanded with no candidate")
	}
	if _, ok := ExpandQuery("anything", "   "); ok {
		t.Error("ExpandQuery() expanded from a blank title")
	}
}

// Prefixing a title the question already contains adds nothing but tokens.
func TestExpandQueryRefusesRedundantTitle(t *testing.T) {
	if _, ok := ExpandQuery("what does the Key rotation runbook say", "Key rotation runbook"); ok {
		t.Error("ExpandQuery() repeated a title already in the question")
	}
}
