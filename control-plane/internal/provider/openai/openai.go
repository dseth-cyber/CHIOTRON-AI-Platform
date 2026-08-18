// Package openai adapts any OpenAI-compatible chat completions API.
//
// One adapter covers OpenAI, Azure OpenAI, Groq, Together, OpenRouter,
// DeepSeek, Mistral, vLLM, NVIDIA NIM and LM Studio, because they all speak the
// same wire format. That is the point of the provider contract: a new backend
// is a row in the providers table, not a package (ARCHITECTURE-v1 section 26).
//
// Nothing here leaks upwards. Business code sees provider.LLM; the vendor's
// field names stop at this file.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// Client speaks the OpenAI chat completions and embeddings API.
type Client struct {
	name       string
	baseURL    string
	credential string
	http       *http.Client
	// embedModel and dimensions are set only when this client is used as an
	// EmbeddingProvider, which is a separate decision from serving completions.
	embedModel string
	dimensions int
}

// New builds an adapter. The name is the provider slug, so the registry and the
// audit trail identify the deployment's own name for this backend rather than
// the vendor's.
func New(name, baseURL, credential string, timeout time.Duration) *Client {
	return &Client{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		credential: credential,
		http:       &http.Client{Timeout: timeout},
	}
}

// WithEmbedding returns a copy configured to serve embeddings too.
func (c *Client) WithEmbedding(model string, dimensions int) *Client {
	clone := *c
	clone.embedModel = model
	clone.dimensions = dimensions
	return &clone
}

func (c *Client) Name() string    { return c.name }
func (c *Client) Model() string   { return c.embedModel }
func (c *Client) Dimensions() int { return c.dimensions }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatBody struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature *float64      `json:"temperature,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	// StreamOptions asks for a usage block on the final streamed chunk. Without
	// it most implementations stream no usage at all and token accounting would
	// silently record zero for every streamed call.
	StreamOptions *streamOptions `json:"stream_options,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type usageBody struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type choiceBody struct {
	Message      chatMessage `json:"message"`
	Delta        chatMessage `json:"delta"`
	FinishReason string      `json:"finish_reason"`
}

type completionBody struct {
	Model   string       `json:"model"`
	Choices []choiceBody `json:"choices"`
	Usage   *usageBody   `json:"usage"`
}

func (c *Client) request(ctx context.Context, path string, body any) (*http.Request, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.credential != "" {
		request.Header.Set("Authorization", "Bearer "+c.credential)
	}
	return request, nil
}

// fail turns a transport or status failure into the right kind of error.
//
// A refused or unreachable host is ErrUnavailable, which the gateway reports as
// a 503 about the compute plane. A 4xx is the caller's request being wrong and
// must not be dressed up as an outage.
func fail(response *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %v", provider.ErrUnavailable, err)
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2<<10))
	detail := strings.TrimSpace(string(body))
	if response.StatusCode >= 500 {
		return fmt.Errorf("%w: upstream returned %d: %s", provider.ErrUnavailable, response.StatusCode, detail)
	}
	return fmt.Errorf("upstream rejected the request with %d: %s", response.StatusCode, detail)
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	started := time.Now()
	request, err := c.request(ctx, "/chat/completions", chatBody{
		Model:       req.Model,
		Messages:    toMessages(req.Messages),
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	})
	if err != nil {
		return provider.ChatResponse{}, err
	}

	response, err := c.http.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			defer response.Body.Close()
		}
		return provider.ChatResponse{}, fail(response, err)
	}
	defer response.Body.Close()

	var decoded completionBody
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return provider.ChatResponse{}, fmt.Errorf("upstream returned no choices")
	}

	return provider.ChatResponse{
		Model:        decoded.Model,
		Content:      decoded.Choices[0].Message.Content,
		FinishReason: decoded.Choices[0].FinishReason,
		Usage:        toUsage(decoded.Usage),
		LatencyMs:    time.Since(started).Milliseconds(),
	}, nil
}

// ChatStream reads the SSE body and emits chunks as they arrive.
func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest,
	emit func(provider.Chunk) error) (provider.ChatResponse, error) {

	started := time.Now()
	request, err := c.request(ctx, "/chat/completions", chatBody{
		Model:         req.Model,
		Messages:      toMessages(req.Messages),
		Temperature:   req.Temperature,
		MaxTokens:     req.MaxTokens,
		Stream:        true,
		StreamOptions: &streamOptions{IncludeUsage: true},
	})
	if err != nil {
		return provider.ChatResponse{}, err
	}
	request.Header.Set("Accept", "text/event-stream")

	response, err := c.http.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			defer response.Body.Close()
		}
		return provider.ChatResponse{}, fail(response, err)
	}
	defer response.Body.Close()

	assembled := provider.ChatResponse{Model: req.Model}
	var content strings.Builder

	scanner := bufio.NewScanner(response.Body)
	// A single SSE line can carry a whole content block, so the default 64 KiB
	// token limit is not enough for every upstream.
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var frame completionBody
		if err := json.Unmarshal([]byte(payload), &frame); err != nil {
			// A frame this adapter cannot read is skipped rather than fatal: a
			// vendor extension must not end somebody's answer mid-sentence.
			continue
		}
		// The usage frame carries no choices, so it must be read before the
		// choice check below rather than after it.
		if frame.Usage != nil {
			assembled.Usage = toUsage(frame.Usage)
		}
		if frame.Model != "" {
			assembled.Model = frame.Model
		}
		if len(frame.Choices) == 0 {
			continue
		}
		if delta := frame.Choices[0].Delta.Content; delta != "" {
			content.WriteString(delta)
			if err := emit(provider.Chunk{Content: delta}); err != nil {
				return provider.ChatResponse{}, err
			}
		}
		if reason := frame.Choices[0].FinishReason; reason != "" {
			assembled.FinishReason = reason
		}
	}
	if err := scanner.Err(); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("%w: reading stream: %v", provider.ErrUnavailable, err)
	}

	assembled.Content = content.String()
	assembled.LatencyMs = time.Since(started).Milliseconds()
	if err := emit(provider.Chunk{
		Done: true, FinishReason: assembled.FinishReason, Usage: &assembled.Usage,
	}); err != nil {
		return provider.ChatResponse{}, err
	}
	return assembled, nil
}

type embeddingBody struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (c *Client) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if c.embedModel == "" {
		return nil, fmt.Errorf("provider %q is not configured for embeddings", c.name)
	}
	request, err := c.request(ctx, "/embeddings", embeddingBody{Model: c.embedModel, Input: inputs})
	if err != nil {
		return nil, err
	}

	response, err := c.http.Do(request)
	if err != nil || response.StatusCode != http.StatusOK {
		if response != nil {
			defer response.Body.Close()
		}
		return nil, fail(response, err)
	}
	defer response.Body.Close()

	var decoded embeddingResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode embeddings: %w", err)
	}
	if len(decoded.Data) != len(inputs) {
		return nil, fmt.Errorf("asked for %d embeddings and received %d", len(inputs), len(decoded.Data))
	}

	vectors := make([][]float32, 0, len(decoded.Data))
	for _, entry := range decoded.Data {
		// A vector of the wrong width would be written into a vector(n) column
		// and fail there, far from the cause.
		if c.dimensions > 0 && len(entry.Embedding) != c.dimensions {
			return nil, fmt.Errorf("provider returned a %d-dimension vector, expected %d",
				len(entry.Embedding), c.dimensions)
		}
		vectors = append(vectors, entry.Embedding)
	}
	return vectors, nil
}

type modelListResponse struct {
	Data []struct {
		ID      string `json:"id"`
		OwnedBy string `json:"owned_by"`
	} `json:"data"`
}

// Models lists what the backend offers.
//
// A backend without /models is not unhealthy: OpenAI has one, several
// self-hosted gateways do not, and refusing to route to them for that would be
// this adapter inventing a requirement the contract does not have.
func (c *Client) Models(ctx context.Context) ([]provider.Model, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if c.credential != "" {
		request.Header.Set("Authorization", "Bearer "+c.credential)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", provider.ErrUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, fail(response, nil)
	}

	var decoded modelListResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode models: %w", err)
	}
	models := make([]provider.Model, 0, len(decoded.Data))
	for _, entry := range decoded.Data {
		models = append(models, provider.Model{ID: entry.ID, Family: entry.OwnedBy})
	}
	return models, nil
}

// Health checks that the credential is accepted and the host answers.
func (c *Client) Health(ctx context.Context) error {
	if _, err := c.Models(ctx); err != nil {
		// A rejected credential is a configuration problem, not an outage, and an
		// operator needs to see the difference on the providers page.
		if errors.Is(err, provider.ErrUnavailable) {
			return err
		}
		return err
	}
	return nil
}

func toMessages(messages []provider.Message) []chatMessage {
	converted := make([]chatMessage, 0, len(messages))
	for _, message := range messages {
		converted = append(converted, chatMessage{Role: message.Role, Content: message.Content})
	}
	return converted
}

func toUsage(usage *usageBody) provider.Usage {
	if usage == nil {
		return provider.Usage{}
	}
	return provider.Usage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
	}
}
