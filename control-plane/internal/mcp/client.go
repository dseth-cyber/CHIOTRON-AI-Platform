// Package mcp is a governed client for Model Context Protocol servers.
//
// MCP servers are reached over the network, never spawned as child processes:
// the Control Plane image ships no runtime to spawn them with, and a tool that
// runs inside the gateway is a tool that shares its blast radius. Servers are
// separate deployables the platform is a client of.
//
// Nothing here decides whether a call is allowed. Authorization, rate limiting
// and audit belong to the tool registry, so a remote tool travels the same
// governance path as a local one (ARCHITECTURE-v1 section 5).
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// protocolVersion is the revision this client implements. A server that speaks a
// different one is refused at discovery rather than surprising a caller later.
const protocolVersion = "2025-06-18"

// maxErrorBody caps how much of a failure response is read into an error. A
// server's output may contain user content.
const maxErrorBody = 2 << 10

var (
	// ErrProtocol is a response that does not follow JSON-RPC or MCP.
	ErrProtocol = errors.New("mcp protocol error")
	// ErrRemote is an error the server itself reported.
	ErrRemote = errors.New("mcp server error")
	// ErrUnavailable is a server that could not be reached.
	ErrUnavailable = errors.New("mcp server unavailable")
)

// Tool is a remote tool as advertised by a server.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// Content is one piece of a tool result. Only text is understood today; other
// kinds are reported rather than silently dropped, because a caller acting on a
// partial result is worse than one told the result was not usable.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallResult is what a tool returned.
type CallResult struct {
	Content []Content `json:"content"`
	// IsError marks a failure the tool itself reported, as opposed to a transport
	// or protocol failure. It is not an exception: the model is meant to see it.
	IsError bool `json:"isError"`
}

// Text flattens the textual content, and says what it could not render.
func (r CallResult) Text() string {
	var parts []string
	skipped := map[string]int{}
	for _, item := range r.Content {
		if item.Type == "text" {
			parts = append(parts, item.Text)
			continue
		}
		skipped[item.Type]++
	}
	for kind, count := range skipped {
		parts = append(parts, fmt.Sprintf("[%d %s item(s) omitted: this client renders text only]", count, kind))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// Server is the connection details for one registered server.
type Server struct {
	Slug    string
	Name    string
	BaseURL string
	Headers map[string]string
	Timeout time.Duration
}

// Client speaks JSON-RPC 2.0 over the Streamable HTTP transport.
type Client struct {
	server Server
	http   *http.Client

	mu          sync.Mutex
	nextID      int
	sessionID   string
	initialised bool
}

func NewClient(server Server) *Client {
	timeout := server.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		server: server,
		http: &http.Client{
			Timeout: timeout,
			// Outbound calls appear as child spans of the request that caused
			// them, which is what makes a slow tool attributable.
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
	}
}

func (c *Client) Slug() string { return c.server.Slug }

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      *int   `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

// Initialise performs the MCP handshake. It is idempotent: repeated calls after
// a successful handshake do nothing, so discovery and a first tool call do not
// race to initialise twice.
func (c *Client) Initialise(ctx context.Context) error {
	c.mu.Lock()
	if c.initialised {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "chiotron-ai-control-plane",
			"version": "1",
		},
	}, &result); err != nil {
		return err
	}
	if result.ProtocolVersion == "" {
		return fmt.Errorf("%w: server %q returned no protocol version", ErrProtocol, c.server.Slug)
	}

	// The notification tells the server the handshake is complete. It carries no
	// id and expects no reply.
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		return err
	}

	c.mu.Lock()
	c.initialised = true
	c.mu.Unlock()
	return nil
}

// ListTools asks what the server offers.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.Initialise(ctx); err != nil {
		return nil, err
	}
	var result struct {
		Tools []Tool `json:"tools"`
	}
	if err := c.call(ctx, "tools/list", map[string]any{}, &result); err != nil {
		return nil, err
	}
	return result.Tools, nil
}

// Call invokes a remote tool.
func (c *Client) Call(ctx context.Context, name string, arguments map[string]any) (CallResult, error) {
	if err := c.Initialise(ctx); err != nil {
		return CallResult{}, err
	}
	if arguments == nil {
		arguments = map[string]any{}
	}
	var result CallResult
	if err := c.call(ctx, "tools/call", map[string]any{
		"name": name, "arguments": arguments,
	}, &result); err != nil {
		return CallResult{}, err
	}
	return result, nil
}

func (c *Client) call(ctx context.Context, method string, params any, out any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	response, err := c.post(ctx, rpcRequest{JSONRPC: "2.0", ID: &id, Method: method, Params: params})
	if err != nil {
		return err
	}
	if response.Error != nil {
		return fmt.Errorf("%w: %s (code %d)", ErrRemote, response.Error.Message, response.Error.Code)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(response.Result, out); err != nil {
		return fmt.Errorf("%w: decode %s result: %s", ErrProtocol, method, err)
	}
	return nil
}

func (c *Client) notify(ctx context.Context, method string) error {
	_, err := c.post(ctx, rpcRequest{JSONRPC: "2.0", Method: method})
	return err
}

func (c *Client) post(ctx context.Context, message rpcRequest) (rpcResponse, error) {
	body, err := json.Marshal(message)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("encode %s request: %w", message.Method, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.server.BaseURL, bytes.NewReader(body))
	if err != nil {
		return rpcResponse{}, fmt.Errorf("build %s request: %w", message.Method, err)
	}
	request.Header.Set("Content-Type", "application/json")
	// Streamable HTTP lets a server answer with either JSON or an SSE stream, so
	// both are accepted and the response is read by content type.
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", protocolVersion)
	for key, value := range c.server.Headers {
		request.Header.Set(key, value)
	}

	c.mu.Lock()
	session := c.sessionID
	c.mu.Unlock()
	if session != "" {
		request.Header.Set("Mcp-Session-Id", session)
	}

	response, err := c.http.Do(request)
	if err != nil {
		return rpcResponse{}, fmt.Errorf("%w: call %s on %q: %s", ErrUnavailable, message.Method, c.server.Slug, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
	}()

	// The server assigns a session on initialize and expects it back on every
	// later request.
	if assigned := response.Header.Get("Mcp-Session-Id"); assigned != "" {
		c.mu.Lock()
		c.sessionID = assigned
		c.mu.Unlock()
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return rpcResponse{}, fmt.Errorf("%w: %s returned %s: %s",
			ErrUnavailable, message.Method, response.Status, strings.TrimSpace(string(detail)))
	}
	// A notification is answered with 202 and no body.
	if message.ID == nil || response.StatusCode == http.StatusAccepted {
		return rpcResponse{}, nil
	}

	payload, err := readPayload(response)
	if err != nil {
		return rpcResponse{}, err
	}

	var decoded rpcResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return rpcResponse{}, fmt.Errorf("%w: decode %s response: %s", ErrProtocol, message.Method, err)
	}
	if decoded.JSONRPC != "2.0" {
		return rpcResponse{}, fmt.Errorf("%w: %s answered with jsonrpc %q", ErrProtocol, message.Method, decoded.JSONRPC)
	}
	return decoded, nil
}

// readPayload handles both transport shapes: a plain JSON body, or an SSE stream
// whose data frames carry the JSON-RPC message.
func readPayload(response *http.Response) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %s", ErrProtocol, err)
	}
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return body, nil
	}

	// Take the last data frame: a server may send progress notifications before
	// the result, and the result is what was asked for.
	var payload string
	for _, line := range strings.Split(string(body), "\n") {
		if data, ok := strings.CutPrefix(strings.TrimRight(line, "\r"), "data: "); ok {
			payload = data
		}
	}
	if payload == "" {
		return nil, fmt.Errorf("%w: event stream carried no data frame", ErrProtocol)
	}
	return []byte(payload), nil
}
