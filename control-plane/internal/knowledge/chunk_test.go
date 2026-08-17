package knowledge

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNewChunkPlanRejectsUnworkableSizes(t *testing.T) {
	cases := map[string]struct{ size, overlap int }{
		"zero size":           {0, 0},
		"negative overlap":    {100, -1},
		"overlap equals size": {100, 100},
		"overlap over size":   {100, 200},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewChunkPlan(tc.size, tc.overlap); err == nil {
				t.Fatalf("NewChunkPlan(%d, %d) succeeded, want error", tc.size, tc.overlap)
			}
		})
	}
}

func TestSupportsMime(t *testing.T) {
	for _, accepted := range []string{"text/plain", "text/markdown", "TEXT/PLAIN", "text/plain; charset=utf-8"} {
		if !SupportsMime(accepted) {
			t.Errorf("SupportsMime(%q) = false, want true", accepted)
		}
	}
	for _, rejected := range []string{"application/pdf", "image/png", ""} {
		if SupportsMime(rejected) {
			t.Errorf("SupportsMime(%q) = true, want false", rejected)
		}
	}
}

func TestParse(t *testing.T) {
	text, err := Parse("text/markdown", []byte("# Title\r\n\r\nBody\r"))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if strings.Contains(text, "\r") {
		t.Errorf("Parse() left carriage returns in %q", text)
	}
}

func TestParseRejectsUnusableContent(t *testing.T) {
	if _, err := Parse("application/pdf", []byte("%PDF-1.4")); err == nil {
		t.Error("Parse() accepted an unsupported type")
	}
	// Storing mojibake would poison the corpus silently.
	if _, err := Parse("text/plain", []byte{0xff, 0xfe, 0x00}); err == nil {
		t.Error("Parse() accepted invalid UTF-8")
	}
	if _, err := Parse("text/plain", []byte("   \n\n  ")); err == nil {
		t.Error("Parse() accepted whitespace-only content")
	}
}

func TestSplitKeepsParagraphsTogether(t *testing.T) {
	plan, err := NewChunkPlan(200, 0)
	if err != nil {
		t.Fatalf("NewChunkPlan() returned error: %v", err)
	}

	text := strings.Join([]string{
		strings.Repeat("a", 90),
		strings.Repeat("b", 90),
		strings.Repeat("c", 90),
	}, "\n\n")

	chunks := plan.Split(text)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2 (two paragraphs then one)", len(chunks))
	}
	if !strings.Contains(chunks[0], "aaa") || !strings.Contains(chunks[0], "bbb") {
		t.Errorf("first chunk did not pack two paragraphs: %q", chunks[0][:20])
	}
}

// An oversized paragraph cannot be kept whole, and cutting it must not split a
// multi-byte character.
func TestSplitCutsOversizedParagraphOnRuneBoundaries(t *testing.T) {
	plan, err := NewChunkPlan(50, 10)
	if err != nil {
		t.Fatalf("NewChunkPlan() returned error: %v", err)
	}

	chunks := plan.Split(strings.Repeat("ก", 200))
	if len(chunks) < 4 {
		t.Fatalf("got %d chunks, want the paragraph cut into several", len(chunks))
	}
	for i, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Errorf("chunk %d is not valid UTF-8", i)
		}
		if strings.ContainsRune(chunk, '�') {
			t.Errorf("chunk %d contains a replacement character: %q", i, chunk)
		}
	}
}

// Overlap is what makes a sentence spanning a boundary retrievable from either
// side.
func TestSplitAppliesOverlap(t *testing.T) {
	plan, err := NewChunkPlan(100, 20)
	if err != nil {
		t.Fatalf("NewChunkPlan() returned error: %v", err)
	}

	first := strings.Repeat("x", 80)
	second := strings.Repeat("y", 80)
	chunks := plan.Split(first + "\n\n" + second)
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if !strings.HasPrefix(chunks[1], strings.Repeat("x", 20)) {
		t.Errorf("second chunk does not start with the previous tail: %q", chunks[1][:30])
	}
}

func TestSplitWithoutOverlapDoesNotRepeat(t *testing.T) {
	plan, err := NewChunkPlan(100, 0)
	if err != nil {
		t.Fatalf("NewChunkPlan() returned error: %v", err)
	}

	chunks := plan.Split(strings.Repeat("x", 80) + "\n\n" + strings.Repeat("y", 80))
	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if strings.Contains(chunks[1], "x") {
		t.Errorf("second chunk repeats the first without overlap configured: %q", chunks[1][:30])
	}
}

// Ordinals are what a citation points at, so the same input must chunk the same
// way every time.
func TestSplitIsDeterministic(t *testing.T) {
	plan, err := NewChunkPlan(120, 20)
	if err != nil {
		t.Fatalf("NewChunkPlan() returned error: %v", err)
	}

	text := "alpha\n\nbeta\n\n" + strings.Repeat("gamma ", 60)
	first := plan.Split(text)
	second := plan.Split(text)

	if len(first) != len(second) {
		t.Fatalf("chunk counts differ between runs: %d and %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("chunk %d differs between runs", i)
		}
	}
}

func TestSplitIgnoresBlankParagraphs(t *testing.T) {
	plan, err := NewChunkPlan(200, 0)
	if err != nil {
		t.Fatalf("NewChunkPlan() returned error: %v", err)
	}

	chunks := plan.Split("one\n\n\n\n   \n\ntwo")
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}
	if strings.Count(chunks[0], "\n\n") != 1 {
		t.Errorf("blank paragraphs survived: %q", chunks[0])
	}
}
