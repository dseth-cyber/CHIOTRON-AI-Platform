package mcp

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ErrInvalidArguments is a call refused before it left the platform.
var ErrInvalidArguments = errors.New("invalid tool arguments")

// Validate checks arguments against a remote tool's declared JSON Schema.
//
// The platform validates before calling rather than letting the server reject
// the call, for two reasons. A malformed call still crosses a trust boundary and
// still costs a rate-limit slot. And an unknown property is more likely to be a
// caller mistake than a feature: forwarding it silently is how a typo becomes an
// argument the server interprets some other way.
//
// This covers the subset of JSON Schema an MCP tool realistically declares:
// object type, properties with primitive types, required, and enum. Anything it
// does not understand is passed through rather than rejected, because refusing a
// legitimate call over an unsupported keyword is worse than the server having to
// check it too.
func Validate(schema map[string]any, arguments map[string]any) error {
	if len(schema) == 0 {
		// A tool with no declared schema is taken at its word; there is nothing to
		// check against.
		return nil
	}

	properties, _ := schema["properties"].(map[string]any)
	if properties == nil {
		return nil
	}

	var problems []string

	for _, name := range requiredNames(schema) {
		value, present := arguments[name]
		if !present || value == nil {
			problems = append(problems, fmt.Sprintf("%q is required", name))
			continue
		}
		if text, ok := value.(string); ok && strings.TrimSpace(text) == "" {
			problems = append(problems, fmt.Sprintf("%q must not be empty", name))
		}
	}

	names := make([]string, 0, len(arguments))
	for name := range arguments {
		names = append(names, name)
	}
	// Sorted so the same bad call always reports the same message, which makes a
	// failure reproducible.
	sort.Strings(names)

	for _, name := range names {
		definition, declared := properties[name].(map[string]any)
		if !declared {
			problems = append(problems, fmt.Sprintf("%q is not an argument of this tool", name))
			continue
		}
		if err := checkValue(name, definition, arguments[name]); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidArguments, strings.Join(problems, "; "))
	}
	return nil
}

func requiredNames(schema map[string]any) []string {
	raw, ok := schema["required"].([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for _, entry := range raw {
		if name, ok := entry.(string); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func checkValue(name string, definition map[string]any, value any) error {
	if value == nil {
		return nil
	}

	if declared, ok := definition["type"].(string); ok {
		if !matchesType(declared, value) {
			return fmt.Errorf("%q must be a %s", name, declared)
		}
	}

	// enum is worth enforcing because it is how an MCP tool expresses a closed
	// set, and sending a value outside it is a caller error the server may or may
	// not catch.
	if options, ok := definition["enum"].([]any); ok && len(options) > 0 {
		for _, option := range options {
			if option == value {
				return nil
			}
		}
		return fmt.Errorf("%q must be one of %s", name, render(options))
	}
	return nil
}

func matchesType(declared string, value any) bool {
	switch declared {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		// JSON has one number type, so an integer arrives as a float64 and is only
		// an integer if it has no fractional part.
		number, ok := value.(float64)
		return ok && number == float64(int64(number))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		// An unrecognised type keyword is not grounds for refusing a call.
		return true
	}
}

func render(options []any) string {
	parts := make([]string, 0, len(options))
	for _, option := range options {
		parts = append(parts, fmt.Sprint(option))
	}
	return strings.Join(parts, ", ")
}
