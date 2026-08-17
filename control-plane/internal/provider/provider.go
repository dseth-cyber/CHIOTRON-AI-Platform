// Package provider defines the Control Plane's vendor-neutral compute
// contract.
//
// Business services depend on these interfaces, never on a vendor SDK
// (ARCHITECTURE-v1 section 4). Swapping Ollama for vLLM, NIM or an external
// API must be an adapter change and a configuration change, nothing else.
package provider

import (
	"context"
	"errors"
)

// ErrUnavailable reports that a compute provider could not be reached. It is
// deliberately distinct from a request error: losing VM5 degrades model calls
// only and must never be reported as a Control Plane failure
// (ARCHITECTURE-v1 section 9).
var ErrUnavailable = errors.New("compute provider unavailable")

// ErrUnknownModel reports that no route exists for a logical model id.
var ErrUnknownModel = errors.New("unknown model")

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest carries an upstream model name. Callers name a logical model and
// the Registry resolves it; adapters never see logical ids.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature *float64
	MaxTokens   *int
}

type Usage struct {
	PromptTokens     int `json:"promptTokens"`
	CompletionTokens int `json:"completionTokens"`
	TotalTokens      int `json:"totalTokens"`
}

type ChatResponse struct {
	Model        string `json:"model"`
	Content      string `json:"content"`
	FinishReason string `json:"finishReason,omitempty"`
	Usage        Usage  `json:"usage"`
	LatencyMs    int64  `json:"latencyMs"`
}

// Model describes what a provider currently has loaded or cached.
type Model struct {
	ID            string `json:"id"`
	Family        string `json:"family,omitempty"`
	ParameterSize string `json:"parameterSize,omitempty"`
	Quantization  string `json:"quantization,omitempty"`
	SizeBytes     int64  `json:"sizeBytes,omitempty"`
}

// Chunk is one increment of a streamed response. Usage and FinishReason are
// populated on the final chunk only, because that is when the provider knows
// them.
type Chunk struct {
	Content      string `json:"content,omitempty"`
	Done         bool   `json:"done,omitempty"`
	FinishReason string `json:"finishReason,omitempty"`
	Usage        *Usage `json:"usage,omitempty"`
}

// LLM is the only surface business code may depend on for text generation.
type LLM interface {
	Name() string
	Health(ctx context.Context) error
	Models(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}

// StreamingLLM is implemented by providers that can emit a response
// incrementally. It is optional: a provider without it still works, and the
// Registry degrades to a single chunk rather than refusing the request.
//
// emit returns an error when the client has gone away; the adapter must stop
// and return it rather than draining the rest of the upstream response.
// ChatStream returns the assembled response so usage accounting does not have
// to be duplicated per transport.
type StreamingLLM interface {
	LLM
	ChatStream(ctx context.Context, req ChatRequest, emit func(Chunk) error) (ChatResponse, error)
}
