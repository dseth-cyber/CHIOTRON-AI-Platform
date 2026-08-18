package anthropic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

func TestChatLiftsTheSystemPromptOutOfTheMessages(t *testing.T) {
	var seen messagesBody
	var seenKey, seenVersion string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.Header.Get("x-api-key")
		seenVersion = r.Header.Get("anthropic-version")
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_, _ = io.WriteString(w, `{"model":"claude-test","content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":3}}`)
	}))
	defer server.Close()

	client := New("claude", server.URL, "sk-ant-secret", 5*time.Second)
	response, err := client.Chat(context.Background(), provider.ChatRequest{
		Model: "claude-test",
		Messages: []provider.Message{
			{Role: "system", Content: "Be concise."},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}

	if seenKey != "sk-ant-secret" || seenVersion != apiVersion {
		t.Errorf("headers = %q / %q, want the credential and the pinned version", seenKey, seenVersion)
	}
	// The system prompt is a top-level field here. Left as a message it would be
	// rejected, and assistant policy would silently stop applying.
	if seen.System != "Be concise." {
		t.Errorf("system = %q, want it lifted out of the messages", seen.System)
	}
	if len(seen.Messages) != 1 || seen.Messages[0].Role != "user" {
		t.Errorf("messages = %+v, want only the user turn", seen.Messages)
	}
	// The API requires max_tokens, so the adapter has to supply one.
	if seen.MaxTokens == 0 {
		t.Error("max_tokens was not set, and the API requires it")
	}
	if response.Content != "hello" || response.Usage.TotalTokens != 10 {
		t.Errorf("response = %+v, want the content and summed usage", response)
	}
}

func TestChatJoinsSeveralSystemMessages(t *testing.T) {
	var seen messagesBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"ok"}]}`)
	}))
	defer server.Close()

	client := New("claude", server.URL, "sk", time.Second)
	_, err := client.Chat(context.Background(), provider.ChatRequest{
		Messages: []provider.Message{
			{Role: "system", Content: "First."},
			{Role: "system", Content: "Second."},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}
	// Dropping either would discard assistant policy without saying so.
	if !strings.Contains(seen.System, "First.") || !strings.Contains(seen.System, "Second.") {
		t.Fatalf("system = %q, want both system messages kept", seen.System)
	}
}

func TestChatStreamAssemblesUsageFromTwoFrames(t *testing.T) {
	// Input tokens arrive on message_start and output tokens on message_delta,
	// so usage has to be built from both rather than read from one.
	frames := []string{
		`data: {"type":"message_start","message":{"model":"claude-test","usage":{"input_tokens":7,"output_tokens":0}}}`,
		`data: {"type":"content_block_delta","delta":{"text":"Hel"}}`,
		`data: {"type":"content_block_delta","delta":{"text":"lo"}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`,
		`data: {"type":"message_stop"}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range frames {
			_, _ = io.WriteString(w, frame+"\n\n")
		}
	}))
	defer server.Close()

	client := New("claude", server.URL, "sk", 5*time.Second)
	var streamed []string
	response, err := client.ChatStream(context.Background(),
		provider.ChatRequest{Model: "claude-test", Messages: []provider.Message{{Role: "user", Content: "hi"}}},
		func(chunk provider.Chunk) error {
			if chunk.Content != "" {
				streamed = append(streamed, chunk.Content)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ChatStream() returned error: %v", err)
	}

	if strings.Join(streamed, "") != "Hello" {
		t.Errorf("streamed %q, want the deltas in order", streamed)
	}
	if response.Usage.PromptTokens != 7 || response.Usage.CompletionTokens != 3 || response.Usage.TotalTokens != 10 {
		t.Errorf("usage = %+v, want 7 in, 3 out, 10 total", response.Usage)
	}
	if response.FinishReason != "end_turn" {
		t.Errorf("finish reason = %q, want end_turn", response.FinishReason)
	}
}

func TestCallerMaxTokensWins(t *testing.T) {
	var seen messagesBody
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_, _ = io.WriteString(w, `{"content":[]}`)
	}))
	defer server.Close()

	limit := 128
	client := New("claude", server.URL, "sk", time.Second)
	if _, err := client.Chat(context.Background(), provider.ChatRequest{MaxTokens: &limit}); err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}
	if seen.MaxTokens != limit {
		t.Fatalf("max_tokens = %d, want the caller's %d", seen.MaxTokens, limit)
	}
}
