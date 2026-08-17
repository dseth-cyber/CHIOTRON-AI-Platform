// Package ollama adapts the Ollama HTTP API to the Control Plane's provider
// contract.
//
// Nothing outside this package may reference Ollama's request or response
// shapes; that is what keeps vLLM, NIM and external APIs a configuration
// change away (ARCHITECTURE-v1 section 4).
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// maxErrorBody caps how much of an upstream error response is read into an
// error message. Model output can be large and may contain user content.
const maxErrorBody = 2 << 10

type Client struct {
	baseURL string
	http    *http.Client
}

// New builds a client for the compute plane at baseURL. The transport is
// instrumented so calls into VM5 appear as child spans of the request that
// triggered them.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http: &http.Client{
			Timeout:   timeout,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

func (c *Client) Name() string { return "ollama" }

// Health asks for the runtime version. It deliberately does not load a model:
// readiness of the compute plane is about reachability, not warm weights.
func (c *Client) Health(ctx context.Context) error {
	var version struct {
		Version string `json:"version"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/version", nil, &version); err != nil {
		return fmt.Errorf("%w: %s", provider.ErrUnavailable, err)
	}
	return nil
}

func (c *Client) Models(ctx context.Context) ([]provider.Model, error) {
	var payload struct {
		Models []struct {
			Model   string `json:"model"`
			Size    int64  `json:"size"`
			Details struct {
				Family            string `json:"family"`
				ParameterSize     string `json:"parameter_size"`
				QuantizationLevel string `json:"quantization_level"`
			} `json:"details"`
		} `json:"models"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &payload); err != nil {
		return nil, fmt.Errorf("%w: %s", provider.ErrUnavailable, err)
	}

	models := make([]provider.Model, 0, len(payload.Models))
	for _, m := range payload.Models {
		models = append(models, provider.Model{
			ID:            m.Model,
			Family:        m.Details.Family,
			ParameterSize: m.Details.ParameterSize,
			Quantization:  m.Details.QuantizationLevel,
			SizeBytes:     m.Size,
		})
	}
	return models, nil
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		// Streaming belongs to the Gateway phase, where it ships with auth and
		// quota enforcement.
		"stream": false,
	}
	options := map[string]any{}
	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		options["num_predict"] = *req.MaxTokens
	}
	if len(options) > 0 {
		body["options"] = options
	}

	var payload struct {
		Model      string           `json:"model"`
		Message    provider.Message `json:"message"`
		DoneReason string           `json:"done_reason"`
		PromptEval int              `json:"prompt_eval_count"`
		Eval       int              `json:"eval_count"`
	}

	started := time.Now()
	if err := c.do(ctx, http.MethodPost, "/api/chat", body, &payload); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("%w: %s", provider.ErrUnavailable, err)
	}

	return provider.ChatResponse{
		Model:        payload.Model,
		Content:      payload.Message.Content,
		FinishReason: payload.DoneReason,
		Usage: provider.Usage{
			PromptTokens:     payload.PromptEval,
			CompletionTokens: payload.Eval,
			TotalTokens:      payload.PromptEval + payload.Eval,
		},
		LatencyMs: time.Since(started).Milliseconds(),
	}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	endpoint, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return fmt.Errorf("build %s url: %w", path, err)
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s request: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("call %s: %w", path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return fmt.Errorf("%s returned %s: %s", path, response.Status, strings.TrimSpace(string(detail)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}
