package conversation

import (
	"strings"
	"testing"
)

// The title is what makes history readable without asking a model to summarise
// it, so it comes from the opening question.
func TestDeriveTitle(t *testing.T) {
	cases := map[string]struct{ question, want string }{
		"short question":     {"How do I rotate a key?", "How do I rotate a key?"},
		"collapses newlines": {"How do\n  I  rotate\ta key?", "How do I rotate a key?"},
		"empty":              {"   ", "Untitled conversation"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := deriveTitle(tc.question); got != tc.want {
				t.Errorf("deriveTitle(%q) = %q, want %q", tc.question, got, tc.want)
			}
		})
	}
}

func TestDeriveTitleTruncates(t *testing.T) {
	title := deriveTitle(strings.Repeat("a", 200))

	if !strings.HasSuffix(title, "…") {
		t.Errorf("title %q is not marked as truncated", title)
	}
	if runes := len([]rune(title)); runes > titleLength+1 {
		t.Errorf("title is %d runes, want at most %d plus the ellipsis", runes, titleLength)
	}
}

// Truncation must count runes, not bytes: cutting a Thai title mid-character
// would store invalid text.
func TestDeriveTitleCutsOnRuneBoundaries(t *testing.T) {
	title := deriveTitle(strings.Repeat("ก", 200))

	if !strings.ContainsRune(title, 'ก') {
		t.Fatalf("title %q lost its content", title)
	}
	for _, char := range title {
		if char == '�' {
			t.Fatalf("title %q contains a broken character", title)
		}
	}
}

// A conversation id reaches SQL as a uuid cast, so a non-uuid must be rejected
// before the query rather than erroring inside it.
func TestIsUUID(t *testing.T) {
	valid := []string{
		"11111111-1111-1111-1111-111111111111",
		"D9B443AD-5C12-428C-9B50-00D974BAB112",
	}
	for _, value := range valid {
		if !isUUID(value) {
			t.Errorf("isUUID(%q) = false, want true", value)
		}
	}

	invalid := []string{
		"", "not-a-uuid", "11111111-1111-1111-1111-11111111111",
		"11111111-1111-1111-1111-1111111111111",
		"11111111x1111-1111-1111-111111111111",
		"1111111g-1111-1111-1111-111111111111",
		"'; DROP TABLE conversations; --",
	}
	for _, value := range invalid {
		if isUUID(value) {
			t.Errorf("isUUID(%q) = true, want false", value)
		}
	}
}
