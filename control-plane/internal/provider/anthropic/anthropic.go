// Package anthropic adapts the Anthropic Messages API.
//
// It is a separate adapter from the OpenAI-compatible one because the wire
// format genuinely differs: the system prompt is a top-level field rather than
// a message, max_tokens is required, and streaming uses named events instead of
// deltas on a choice. Pretending one adapter covers both would mean a shim that
// breaks quietly whenever either side changes.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// apiVersion pins the wire contract. Anthropic dates its API rather than
// numbering it, and an unpinned client changes behaviour without a deploy.
const apiVersion = "2023-06-01"

// defaultMaxTokens is used when a caller names none, because the API requires
// the field. It is a ceiling, not a target: a shorter answer still stops early.
const defaultMaxTokens = 4096

type Client struct {
	name       string
	baseURL    string
	credential string
	http       *http.Client
}

func New(name, baseURL, credential string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "https://api.anthropic.com"
	}
	return &Client{
		name:       name,
		baseURL:    strings.TrimRight(baseURL, "/"),
		credential: credential,
		http:       &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string { return c.name }

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesBody struct {
	Model string `json:"model"`
	// System is a top-level field here, not a message with role "system".
	System      string    `json:"system,omitempty"`
	Messages    []message `json:"messages"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature *float64  `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type usageBody struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type messagesResponse struct {
	Model      string         `json:"model"`
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      usageBody      `json:"usage"`
}

// split separates the system prompt from the turns.
//
// The platform assembles a system message like every other provider expects, so
// the conversion happens here rather than making callers know which backend
// they are talking to. Several system messages are joined: dropping any would
// silently discard assistant policy.
func split(messages []provider.Message) (string, []message) {
	var system []string
	turns := make([]message, 0, len(messages))
	for _, entry := range messages {
		if entry.Role == "system" {
			system = append(system, entry.Content)
			continue
		}
		turns = append(turns, message{Role: entry.Role, Content: entry.Content})
	}
	return strings.Join(system, "\n\n"), turns
}

func (c *Client) request(ctx context.Context, body any) (*http.Request, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/messages", bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("anthropic-version", apiVersion)
	if c.credential != "" {
		request.Header.Set("x-api-key", c.credential)
	}
	return request, nil
}

func fail(response *http.Response, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %v", provider.ErrUnavailable, err)
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 2<<10))
	detail := strings.TrimSpace(string(body))
	// 429 is a rate limit, not a broken host: reporting it as unavailable would
	// have the gateway blame the compute plane for the caller's own pace.
	if response.StatusCode >= 500 {
		return fmt.Errorf("%w: upstream returned %d: %s", provider.ErrUnavailable, response.StatusCode, detail)
	}
	return fmt.Errorf("upstream rejected the request with %d: %s", response.StatusCode, detail)
}

func (c *Client) body(req provider.ChatRequest, stream bool) messagesBody {
	system, turns := split(req.Messages)
	maxTokens := defaultMaxTokens
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		maxTokens = *req.MaxTokens
	}
	return messagesBody{
		Model: req.Model, System: system, Messages: turns,
		MaxTokens: maxTokens, Temperature: req.Temperature, Stream: stream,
	}
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	started := time.Now()
	request, err := c.request(ctx, c.body(req, false))
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

	var decoded messagesResponse
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("decode response: %w", err)
	}

	var content strings.Builder
	for _, block := range decoded.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}
	return provider.ChatResponse{
		Model:        decoded.Model,
		Content:      content.String(),
		FinishReason: decoded.StopReason,
		Usage:        toUsage(decoded.Usage),
		LatencyMs:    time.Since(started).Milliseconds(),
	}, nil
}

type streamFrame struct {
	Type  string `json:"type"`
	Delta struct {
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"delta"`
	Message struct {
		Model string    `json:"model"`
		Usage usageBody `json:"usage"`
	} `json:"message"`
	Usage usageBody `json:"usage"`
}

// ChatStream reads the named-event SSE stream.
//
// Input tokens arrive on message_start and output tokens on message_delta, so
// usage is assembled from two frames rather than read from one.
func (c *Client) ChatStream(ctx context.Context, req provider.ChatRequest,
	emit func(provider.Chunk) error) (provider.ChatResponse, error) {

	started := time.Now()
	request, err := c.request(ctx, c.body(req, true))
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
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var frame streamFrame
		if err := json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(line, "data:"))), &frame); err != nil {
			continue
		}

		switch frame.Type {
		case "message_start":
			if frame.Message.Model != "" {
				assembled.Model = frame.Message.Model
			}
			assembled.Usage.PromptTokens = frame.Message.Usage.InputTokens
		case "content_block_delta":
			if frame.Delta.Text == "" {
				continue
			}
			content.WriteString(frame.Delta.Text)
			if err := emit(provider.Chunk{Content: frame.Delta.Text}); err != nil {
				return provider.ChatResponse{}, err
			}
		case "message_delta":
			if frame.Delta.StopReason != "" {
				assembled.FinishReason = frame.Delta.StopReason
			}
			assembled.Usage.CompletionTokens = frame.Usage.OutputTokens
		case "error":
			return provider.ChatResponse{}, fmt.Errorf("%w: upstream reported an error mid-stream", provider.ErrUnavailable)
		}
	}
	if err := scanner.Err(); err != nil {
		return provider.ChatResponse{}, fmt.Errorf("%w: reading stream: %v", provider.ErrUnavailable, err)
	}

	assembled.Content = content.String()
	assembled.Usage.TotalTokens = assembled.Usage.PromptTokens + assembled.Usage.CompletionTokens
	assembled.LatencyMs = time.Since(started).Milliseconds()
	if err := emit(provider.Chunk{
		Done: true, FinishReason: assembled.FinishReason, Usage: &assembled.Usage,
	}); err != nil {
		return provider.ChatResponse{}, err
	}
	return assembled, nil
}

// Models returns nothing because the Messages API has no catalogue endpoint.
//
// An empty list is honest here: the platform marks the route available from
// health instead, and inventing a hard-coded model list would go stale the
// week after it was written.
func (c *Client) Models(context.Context) ([]provider.Model, error) { return nil, nil }

// Health sends the smallest possible message.
//
// There is no free probe endpoint, so this costs one token. That is the price
// of knowing whether the credential works before a user finds out.
func (c *Client) Health(ctx context.Context) error {
	request, err := c.request(ctx, messagesBody{
		Model: "claude-3-5-haiku-latest", MaxTokens: 1,
		Messages: []message{{Role: "user", Content: "ping"}},
	})
	if err != nil {
		return err
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", provider.ErrUnavailable, err)
	}
	defer response.Body.Close()

	// A 404 on the model still proves the host answered and the credential was
	// accepted, which is what health is asking about.
	if response.StatusCode == http.StatusOK || response.StatusCode == http.StatusNotFound {
		return nil
	}
	return fail(response, nil)
}

func toUsage(usage usageBody) provider.Usage {
	return provider.Usage{
		PromptTokens:     usage.InputTokens,
		CompletionTokens: usage.OutputTokens,
		TotalTokens:      usage.InputTokens + usage.OutputTokens,
	}
}
