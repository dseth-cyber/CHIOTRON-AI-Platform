// Package ollama adapts the Ollama HTTP API to the Control Plane's provider
// contract.
//
// Nothing outside this package may reference Ollama's request or response
// shapes; that is what keeps vLLM, NIM and external APIs a configuration
// change away (ARCHITECTURE-v1 section 4).
package ollama

import (
	"bufio"
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

// maxStreamLine bounds one newline-delimited chunk so a malformed upstream
// response cannot exhaust memory.
const maxStreamLine = 1 << 20

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

// chatPayload is one Ollama /api/chat response. In streaming mode the endpoint
// returns a sequence of these as newline-delimited JSON, with the token counts
// carried only on the final object.
type chatPayload struct {
	Model      string           `json:"model"`
	Message    provider.Message `json:"message"`
	Done       bool             `json:"done"`
	DoneReason string           `json:"done_reason"`
	PromptEval int              `json:"prompt_eval_count"`
	Eval       int              `json:"eval_count"`
}

func chatBody(req provider.ChatRequest, stream bool) map[string]any {
	body := map[string]any{
		"model":    req.Model,
		"messages": req.Messages,
		"stream":   stream,
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
	return body
}

func (p chatPayload) response(content string, latency time.Duration) provider.ChatResponse {
	return provider.ChatResponse{
		Model:        p.Model,
		Content:      content,
		FinishReason: p.DoneReason,
		Usage: provider.Usage{
			PromptTokens:     p.PromptEval,
			CompletionTokens: p.Eval,
			TotalTokens:      p.PromptEval + p.Eval,
		},
		LatencyMs: latency.Milliseconds(),
	}
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	var payload chatPayload

	started := time.Now()
	if err := c.do(ctx, http.MethodPost, "/api/chat", chatBody(req, false), &payload); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("%w: %s", provider.ErrUnavailable, err)
	}
	return payload.response(payload.Message.Content, time.Since(started)), nil
}

// ChatStream reads Ollama's newline-delimited JSON stream and emits each
// increment as it arrives.
func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest, emit func(provider.Chunk) error) (provider.ChatResponse, error) {
	started := time.Now()
	response, err := c.post(ctx, "/api/chat", chatBody(req, true))
	if err != nil {
		return provider.ChatResponse{}, fmt.Errorf("%w: %s", provider.ErrUnavailable, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	var content strings.Builder
	var final chatPayload

	scanner := bufio.NewScanner(response.Body)
	// A single token is small, but a provider is free to buffer; give the
	// scanner room rather than failing a long line.
	scanner.Buffer(make([]byte, 0, 64<<10), maxStreamLine)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var payload chatPayload
		if err := json.Unmarshal(line, &payload); err != nil {
			return provider.ChatResponse{}, fmt.Errorf("%w: decode stream chunk: %s", provider.ErrUnavailable, err)
		}

		if payload.Message.Content != "" {
			content.WriteString(payload.Message.Content)
			// An emit failure means the client is gone. Stop rather than
			// draining the rest of the upstream response.
			if err := emit(provider.Chunk{Content: payload.Message.Content}); err != nil {
				return provider.ChatResponse{}, err
			}
		}
		if payload.Done {
			final = payload
			if final.Model == "" {
				final.Model = req.Model
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("%w: read stream: %s", provider.ErrUnavailable, err)
	}
	if !final.Done {
		// The connection ended before the provider said it was finished, so the
		// answer is truncated and must not be reported as a success.
		return provider.ChatResponse{}, fmt.Errorf("%w: stream ended before completion", provider.ErrUnavailable)
	}

	return final.response(content.String(), time.Since(started)), nil
}

// do performs a request and decodes the whole response body.
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	response, err := c.send(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

// post returns the live response so the caller can read a stream. The caller
// owns closing the body.
func (c *Client) post(ctx context.Context, path string, body any) (*http.Response, error) {
	return c.send(ctx, http.MethodPost, path, body)
}

// send issues the request and fails on any non-2xx status, so callers only ever
// see a body worth reading.
func (c *Client) send(ctx context.Context, method, path string, body any) (*http.Response, error) {
	endpoint, err := url.JoinPath(c.baseURL, path)
	if err != nil {
		return nil, fmt.Errorf("build %s url: %w", path, err)
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode %s request: %w", path, err)
		}
		reader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return nil, fmt.Errorf("build %s request: %w", path, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", path, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		_ = response.Body.Close()
		return nil, fmt.Errorf("%s returned %s: %s", path, response.Status, strings.TrimSpace(string(detail)))
	}
	return response, nil
}
