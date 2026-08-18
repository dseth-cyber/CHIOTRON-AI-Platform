package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stubServer is a minimal MCP server: enough of the protocol to exercise the
// client against something that behaves like the real thing.
type stubServer struct {
	tools     []Tool
	result    CallResult
	remoteErr *rpcError
	sessionID string

	methods   []string
	sessions  []string
	lastCall  map[string]any
	asStream  bool
	protocol  string
	initCount int
}

func (s *stubServer) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var request rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("stub server received malformed JSON: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.methods = append(s.methods, request.Method)
		s.sessions = append(s.sessions, r.Header.Get("Mcp-Session-Id"))

		if s.sessionID != "" {
			w.Header().Set("Mcp-Session-Id", s.sessionID)
		}

		// A notification carries no id and is answered with 202 and no body.
		if request.ID == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}

		var result any
		switch request.Method {
		case "initialize":
			s.initCount++
			version := s.protocol
			if version == "" {
				version = protocolVersion
			}
			result = map[string]any{
				"protocolVersion": version,
				"serverInfo":      map[string]any{"name": "stub", "version": "1"},
			}
		case "tools/list":
			result = map[string]any{"tools": s.tools}
		case "tools/call":
			params, _ := request.Params.(map[string]any)
			s.lastCall = params
			result = s.result
		}

		response := map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}
		if s.remoteErr != nil && request.Method == "tools/call" {
			response = map[string]any{"jsonrpc": "2.0", "id": request.ID, "error": s.remoteErr}
		}
		body, _ := json.Marshal(response)

		if s.asStream {
			// Streamable HTTP allows an SSE response, with progress frames before
			// the result.
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\n"))
			_, _ = w.Write([]byte("event: message\ndata: " + string(body) + "\n\n"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}
}

func newTestClient(t *testing.T, server *stubServer) *Client {
	t.Helper()
	httpServer := httptest.NewServer(server.handler(t))
	t.Cleanup(httpServer.Close)
	return NewClient(Server{Slug: "stub", BaseURL: httpServer.URL, Timeout: 5 * time.Second})
}

func searchTool() Tool {
	return Tool{
		Name: "search", Description: "Searches records.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "What to look for."},
				"limit": map[string]any{"type": "integer"},
			},
			"required": []any{"query"},
		},
	}
}

func TestListToolsPerformsTheHandshakeFirst(t *testing.T) {
	server := &stubServer{tools: []Tool{searchTool()}}
	client := newTestClient(t, server)

	tools, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() returned error: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "search" {
		t.Fatalf("tools = %+v, want the advertised tool", tools)
	}

	// The order is the protocol: initialize, the initialized notification, then
	// any request.
	want := []string{"initialize", "notifications/initialized", "tools/list"}
	if strings.Join(server.methods, ",") != strings.Join(want, ",") {
		t.Errorf("methods = %v, want %v", server.methods, want)
	}
}

// Discovery and a first call must not race to initialise twice.
func TestInitialiseIsIdempotent(t *testing.T) {
	server := &stubServer{tools: []Tool{searchTool()}, result: CallResult{Content: []Content{{Type: "text", Text: "ok"}}}}
	client := newTestClient(t, server)

	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools() returned error: %v", err)
	}
	if _, err := client.Call(context.Background(), "search", map[string]any{"query": "x"}); err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if server.initCount != 1 {
		t.Errorf("initialize was called %d times, want once", server.initCount)
	}
}

// The server assigns a session on initialize and expects it back afterwards.
func TestSessionIdIsReturnedOnLaterRequests(t *testing.T) {
	server := &stubServer{tools: []Tool{searchTool()}, sessionID: "session-42"}
	client := newTestClient(t, server)

	if _, err := client.ListTools(context.Background()); err != nil {
		t.Fatalf("ListTools() returned error: %v", err)
	}
	if len(server.sessions) < 3 {
		t.Fatalf("stub saw %d requests, want at least 3", len(server.sessions))
	}
	if server.sessions[0] != "" {
		t.Errorf("initialize carried a session id %q, want none", server.sessions[0])
	}
	for i, session := range server.sessions[1:] {
		if session != "session-42" {
			t.Errorf("request %d carried session %q, want session-42", i+1, session)
		}
	}
}

// Streamable HTTP allows either shape, and a server may send progress frames
// before the result.
func TestCallReadsAnEventStreamResponse(t *testing.T) {
	server := &stubServer{
		tools:    []Tool{searchTool()},
		asStream: true,
		result:   CallResult{Content: []Content{{Type: "text", Text: "streamed answer"}}},
	}
	client := newTestClient(t, server)

	result, err := client.Call(context.Background(), "search", map[string]any{"query": "x"})
	if err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if result.Text() != "streamed answer" {
		t.Errorf("Text() = %q, want the result frame rather than the progress frame", result.Text())
	}
}

func TestCallSendsNameAndArguments(t *testing.T) {
	server := &stubServer{tools: []Tool{searchTool()}, result: CallResult{Content: []Content{{Type: "text", Text: "ok"}}}}
	client := newTestClient(t, server)

	if _, err := client.Call(context.Background(), "search", map[string]any{"query": "invoices"}); err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if server.lastCall["name"] != "search" {
		t.Errorf("call name = %v, want search", server.lastCall["name"])
	}
	arguments, _ := server.lastCall["arguments"].(map[string]any)
	if arguments["query"] != "invoices" {
		t.Errorf("arguments = %v, want the query forwarded", arguments)
	}
}

// A nil argument map must still send an object: a server declaring no required
// arguments should not receive a null.
func TestCallSendsAnObjectForNoArguments(t *testing.T) {
	server := &stubServer{tools: []Tool{searchTool()}, result: CallResult{}}
	client := newTestClient(t, server)

	if _, err := client.Call(context.Background(), "ping", nil); err != nil {
		t.Fatalf("Call() returned error: %v", err)
	}
	if _, ok := server.lastCall["arguments"].(map[string]any); !ok {
		t.Errorf("arguments = %v, want an empty object", server.lastCall["arguments"])
	}
}

func TestRemoteErrorIsDistinctFromTransportFailure(t *testing.T) {
	server := &stubServer{
		tools:     []Tool{searchTool()},
		remoteErr: &rpcError{Code: -32602, Message: "unknown tool"},
	}
	client := newTestClient(t, server)

	_, err := client.Call(context.Background(), "search", map[string]any{"query": "x"})
	if !errors.Is(err, ErrRemote) {
		t.Fatalf("error = %v, want ErrRemote", err)
	}
	if errors.Is(err, ErrUnavailable) {
		t.Error("a server-reported error was classified as unreachable")
	}
}

func TestUnreachableServerIsReported(t *testing.T) {
	client := NewClient(Server{Slug: "stub", BaseURL: "http://127.0.0.1:1", Timeout: time.Second})

	if _, err := client.ListTools(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

// A server speaking something other than JSON-RPC 2.0 is refused rather than
// half-understood.
func TestNonJSONRPCResponseIsRefused(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"1.0","id":1,"result":{}}`))
	}))
	t.Cleanup(httpServer.Close)

	client := NewClient(Server{Slug: "stub", BaseURL: httpServer.URL, Timeout: time.Second})
	if err := client.Initialise(context.Background()); !errors.Is(err, ErrProtocol) {
		t.Fatalf("error = %v, want ErrProtocol", err)
	}
}

// A server that declares no protocol version is refused: continuing would mean
// guessing which revision it speaks.
func TestMissingProtocolVersionIsRefused(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"stub"}}}`))
	}))
	t.Cleanup(httpServer.Close)

	client := NewClient(Server{Slug: "stub", BaseURL: httpServer.URL, Timeout: time.Second})
	if err := client.Initialise(context.Background()); !errors.Is(err, ErrProtocol) {
		t.Fatalf("error = %v, want ErrProtocol", err)
	}
}

// A server declaring a version this client does not implement is still accepted:
// the version is negotiated, and refusing every mismatch would make the client
// useless against a server one revision ahead.
func TestDifferentProtocolVersionIsAccepted(t *testing.T) {
	client := newTestClient(t, &stubServer{protocol: "2024-11-05"})

	if err := client.Initialise(context.Background()); err != nil {
		t.Fatalf("Initialise() refused a different protocol revision: %v", err)
	}
}

// Only text content is understood. Silently dropping the rest would give a
// caller a partial answer that looks complete.
func TestTextReportsUnrenderableContent(t *testing.T) {
	result := CallResult{Content: []Content{
		{Type: "text", Text: "the answer"},
		{Type: "image"},
	}}

	rendered := result.Text()
	if !strings.Contains(rendered, "the answer") {
		t.Errorf("Text() = %q, want the text content", rendered)
	}
	if !strings.Contains(rendered, "omitted") {
		t.Errorf("Text() = %q, want the omitted content declared", rendered)
	}
}

func TestSendsProtocolVersionHeader(t *testing.T) {
	var header string
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header = r.Header.Get("MCP-Protocol-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"` + protocolVersion + `"}}`))
	}))
	t.Cleanup(httpServer.Close)

	client := NewClient(Server{Slug: "stub", BaseURL: httpServer.URL, Timeout: time.Second})
	_ = client.Initialise(context.Background())
	if header != protocolVersion {
		t.Errorf("MCP-Protocol-Version = %q, want %q", header, protocolVersion)
	}
}

// Credentials are configured per server and must reach it.
func TestConfiguredHeadersAreSent(t *testing.T) {
	var authorization string
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"` + protocolVersion + `"}}`))
	}))
	t.Cleanup(httpServer.Close)

	client := NewClient(Server{
		Slug: "stub", BaseURL: httpServer.URL, Timeout: time.Second,
		Headers: map[string]string{"Authorization": "Bearer remote-secret"},
	})
	_ = client.Initialise(context.Background())
	if authorization != "Bearer remote-secret" {
		t.Errorf("Authorization = %q, want the configured credential", authorization)
	}
}
