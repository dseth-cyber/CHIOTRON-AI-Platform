package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/mcp"
)

// Caller is the narrow surface an adapter needs from an MCP client.
type Caller interface {
	Slug() string
	Call(ctx context.Context, name string, arguments map[string]any) (mcp.CallResult, error)
}

// Remote adapts one tool on one MCP server into the registry's contract.
//
// Each remote tool gets its own Implementation with its own Kind, so the
// registry treats it exactly like a built-in: same scope check, same per-tool
// rate limit, same audit row. A remote tool that bypassed any of that would be a
// second governance path, and the weaker one would become the way in.
type Remote struct {
	Client     Caller
	RemoteName string
	// Slug is the registered name, namespaced by server so two servers may both
	// offer a tool called "search".
	Slug        string
	Description string
	Schema      map[string]any
	// Primary is the argument a caller's question maps onto, inferred from the
	// schema at registration time.
	Primary string
}

// Kind is unique per remote tool, which is what lets one registry hold tools
// from several servers without their kinds colliding.
func (r Remote) Kind() string { return "mcp:" + r.Slug }

func (r Remote) PrimaryArgument() string { return r.Primary }

// Describe renders the remote schema as the same shape the built-in tools use.
func (r Remote) Describe() map[string]string {
	described := map[string]string{}
	properties, _ := r.Schema["properties"].(map[string]any)
	required := map[string]bool{}
	if names, ok := r.Schema["required"].([]any); ok {
		for _, name := range names {
			if text, ok := name.(string); ok {
				required[text] = true
			}
		}
	}

	for name, raw := range properties {
		definition, _ := raw.(map[string]any)
		kind, _ := definition["type"].(string)
		if kind == "" {
			kind = "any"
		}
		necessity := "optional"
		if required[name] {
			necessity = "required"
		}
		description, _ := definition["description"].(string)
		described[name] = strings.TrimSpace(fmt.Sprintf("%s, %s. %s", kind, necessity, description))
	}
	return described
}

func (r Remote) Invoke(ctx context.Context, call Invocation) (Result, error) {
	// Validation happens before the call leaves the platform. A malformed call
	// still crosses a trust boundary and still costs a rate-limit slot.
	//
	// The error is re-tagged with the registry's sentinel so the call is recorded
	// as denied rather than failed, carrying only the problem list: both packages
	// name the same condition, and wrapping one in the other would print it twice
	// in the audit row an operator reads.
	if err := mcp.Validate(r.Schema, call.Arguments); err != nil {
		problems := strings.TrimPrefix(err.Error(), mcp.ErrInvalidArguments.Error()+": ")
		return Result{}, fmt.Errorf("%w: %s", ErrInvalidArguments, problems)
	}

	result, err := r.Client.Call(ctx, r.RemoteName, call.Arguments)
	if err != nil {
		return Result{}, err
	}
	if result.IsError {
		// A tool reporting its own failure is not a platform failure. It is
		// returned as content so the model can react, and recorded as a failed
		// call so an operator can see it.
		return Result{Content: result.Text()}, fmt.Errorf("tool %q reported an error: %s", r.Slug, result.Text())
	}
	return Result{Content: result.Text(), Data: result.Content}, nil
}

// InferPrimaryArgument picks the argument a free-text question maps onto.
//
// It prefers a required string, because that is what a single-input tool
// declares, and falls back to conventional names. With nothing obvious it
// returns empty, which makes the tool callable with explicit arguments but not
// from a bare question - better than guessing wrong and sending a question to an
// argument that means something else.
func InferPrimaryArgument(schema map[string]any) string {
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) == 0 {
		return ""
	}

	var required []string
	if names, ok := schema["required"].([]any); ok {
		for _, name := range names {
			if text, ok := name.(string); ok {
				required = append(required, text)
			}
		}
	}

	isString := func(name string) bool {
		definition, ok := properties[name].(map[string]any)
		if !ok {
			return false
		}
		kind, _ := definition["type"].(string)
		return kind == "string"
	}

	// Exactly one required string is unambiguous.
	var requiredStrings []string
	for _, name := range required {
		if isString(name) {
			requiredStrings = append(requiredStrings, name)
		}
	}
	if len(requiredStrings) == 1 {
		return requiredStrings[0]
	}

	for _, conventional := range []string{"query", "question", "term", "text", "input", "prompt", "message"} {
		if isString(conventional) {
			return conventional
		}
	}
	return ""
}
