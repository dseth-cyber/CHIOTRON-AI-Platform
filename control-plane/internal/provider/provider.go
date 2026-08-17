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

// LLM is the only surface business code may depend on for text generation.
//
// Streaming is intentionally absent: SSE belongs to the Gateway phase, where
// it arrives together with authentication and quota enforcement
// (ARCHITECTURE-v1 section 7).
type LLM interface {
	Name() string
	Health(ctx context.Context) error
	Models(ctx context.Context) ([]Model, error)
	Chat(ctx context.Context, req ChatRequest) (ChatResponse, error)
}
