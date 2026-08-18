package mcp

import (
	"errors"
	"strings"
	"testing"
)

func schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query":   map[string]any{"type": "string"},
			"limit":   map[string]any{"type": "integer"},
			"exact":   map[string]any{"type": "boolean"},
			"tags":    map[string]any{"type": "array"},
			"scope":   map[string]any{"type": "string", "enum": []any{"open", "closed"}},
			"ratio":   map[string]any{"type": "number"},
			"unknown": map[string]any{"type": "weird-kind"},
		},
		"required": []any{"query"},
	}
}

func TestValidateAcceptsAWellFormedCall(t *testing.T) {
	err := Validate(schema(), map[string]any{
		"query": "invoices", "limit": float64(5), "exact": true,
		"tags": []any{"a"}, "scope": "open", "ratio": 0.5,
	})
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}
}

func TestValidateRequiresDeclaredArguments(t *testing.T) {
	err := Validate(schema(), map[string]any{"limit": float64(5)})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("error = %v, want ErrInvalidArguments", err)
	}
	if !strings.Contains(err.Error(), "query") {
		t.Errorf("error %q does not name the missing argument", err)
	}
}

// A required string present but blank is a caller mistake, not a value.
func TestValidateRejectsBlankRequiredStrings(t *testing.T) {
	if err := Validate(schema(), map[string]any{"query": "   "}); !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("error = %v, want ErrInvalidArguments", err)
	}
}

func TestValidateChecksTypes(t *testing.T) {
	cases := map[string]map[string]any{
		"string given a number":  {"query": float64(1)},
		"integer given a float":  {"query": "x", "limit": 1.5},
		"boolean given a string": {"query": "x", "exact": "yes"},
		"array given an object":  {"query": "x", "tags": map[string]any{}},
	}
	for name, arguments := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Validate(schema(), arguments); !errors.Is(err, ErrInvalidArguments) {
				t.Fatalf("error = %v, want ErrInvalidArguments", err)
			}
		})
	}
}

// JSON has one number type, so an integer arrives as a float and is only an
// integer if it has no fractional part.
func TestValidateAcceptsWholeNumbersAsIntegers(t *testing.T) {
	if err := Validate(schema(), map[string]any{"query": "x", "limit": float64(7)}); err != nil {
		t.Fatalf("Validate() rejected a whole number as an integer: %v", err)
	}
}

func TestValidateEnforcesEnums(t *testing.T) {
	err := Validate(schema(), map[string]any{"query": "x", "scope": "halfway"})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("error = %v, want ErrInvalidArguments", err)
	}
	if !strings.Contains(err.Error(), "open") {
		t.Errorf("error %q does not list the permitted values", err)
	}
}

// An unknown property is more likely a caller mistake than a feature, and
// forwarding it silently is how a typo becomes an argument the server reads
// some other way.
func TestValidateRejectsUndeclaredArguments(t *testing.T) {
	err := Validate(schema(), map[string]any{"query": "x", "quer": "typo"})
	if !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("error = %v, want ErrInvalidArguments", err)
	}
	if !strings.Contains(err.Error(), "quer") {
		t.Errorf("error %q does not name the unexpected argument", err)
	}
}

// Refusing a legitimate call over a keyword this validator does not implement
// would be worse than letting the server check it too.
func TestValidatePassesThroughUnsupportedTypes(t *testing.T) {
	if err := Validate(schema(), map[string]any{"query": "x", "unknown": []any{1, 2}}); err != nil {
		t.Fatalf("Validate() rejected an unsupported type keyword: %v", err)
	}
}

func TestValidateAcceptsAnythingWithoutASchema(t *testing.T) {
	if err := Validate(nil, map[string]any{"anything": 1}); err != nil {
		t.Fatalf("Validate() returned error for an undeclared schema: %v", err)
	}
	if err := Validate(map[string]any{"type": "object"}, map[string]any{"anything": 1}); err != nil {
		t.Fatalf("Validate() returned error for a schema with no properties: %v", err)
	}
}

// The same bad call must always report the same message, or a failure cannot be
// reproduced from a log line.
func TestValidateReportsProblemsDeterministically(t *testing.T) {
	arguments := map[string]any{"alpha": 1, "beta": 2, "gamma": 3}
	first := Validate(schema(), arguments)
	second := Validate(schema(), arguments)

	if first.Error() != second.Error() {
		t.Errorf("messages differ between runs:\n%v\n%v", first, second)
	}
}
