package openai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

func TestChatSendsTheCredentialAndReadsTheAnswer(t *testing.T) {
	var seenAuth, seenPath string
	var seenBody chatBody

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&seenBody)
		_, _ = io.WriteString(w, `{"model":"gpt-test","choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`)
	}))
	defer server.Close()

	client := New("openai", server.URL, "sk-secret", 5*time.Second)
	response, err := client.Chat(context.Background(), provider.ChatRequest{
		Model:    "gpt-test",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}

	if seenAuth != "Bearer sk-secret" {
		t.Errorf("Authorization = %q, want the bearer credential", seenAuth)
	}
	if seenPath != "/chat/completions" {
		t.Errorf("path = %q, want /chat/completions", seenPath)
	}
	if response.Content != "hello" || response.Usage.TotalTokens != 10 {
		t.Errorf("response = %+v, want the upstream content and usage", response)
	}
}

func TestChatStreamReassemblesDeltasAndUsage(t *testing.T) {
	// Usage arrives on a frame with no choices, which is exactly the shape that
	// breaks a reader that checks for a choice before looking for usage.
	frames := []string{
		`data: {"model":"gpt-test","choices":[{"delta":{"content":"Hel"}}]}`,
		`data: {"choices":[{"delta":{"content":"lo"}}]}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
		`data: {"choices":[],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`,
		`data: [DONE]`,
	}

	var streamRequested bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body chatBody
		_ = json.NewDecoder(r.Body).Decode(&body)
		streamRequested = body.Stream && body.StreamOptions != nil && body.StreamOptions.IncludeUsage
		w.Header().Set("Content-Type", "text/event-stream")
		for _, frame := range frames {
			_, _ = io.WriteString(w, frame+"\n\n")
		}
	}))
	defer server.Close()

	client := New("openai", server.URL, "sk-secret", 5*time.Second)
	var streamed []string
	response, err := client.ChatStream(context.Background(),
		provider.ChatRequest{Model: "gpt-test", Messages: []provider.Message{{Role: "user", Content: "hi"}}},
		func(chunk provider.Chunk) error {
			if chunk.Content != "" {
				streamed = append(streamed, chunk.Content)
			}
			return nil
		})
	if err != nil {
		t.Fatalf("ChatStream() returned error: %v", err)
	}

	// Without stream_options the upstream sends no usage at all and every
	// streamed call would be billed as zero tokens.
	if !streamRequested {
		t.Error("the request did not ask for usage on the stream")
	}
	if strings.Join(streamed, "") != "Hello" {
		t.Errorf("streamed %q, want the deltas in order", streamed)
	}
	if response.Content != "Hello" || response.FinishReason != "stop" {
		t.Errorf("assembled = %+v, want the joined content and finish reason", response)
	}
	if response.Usage.TotalTokens != 10 {
		t.Errorf("usage = %+v, want the totals from the usage-only frame", response.Usage)
	}
}

func TestUpstreamOutageIsUnavailableAndARejectionIsNot(t *testing.T) {
	// The distinction decides whether the gateway answers 503 about the compute
	// plane or 4xx about the caller's request, so it has to be right.
	outage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer outage.Close()

	client := New("openai", outage.URL, "sk", time.Second)
	_, err := client.Chat(context.Background(), provider.ChatRequest{Model: "m"})
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("a 502 gave error %v, want ErrUnavailable", err)
	}

	rejected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":{"message":"unknown model"}}`)
	}))
	defer rejected.Close()

	client = New("openai", rejected.URL, "sk", time.Second)
	_, err = client.Chat(context.Background(), provider.ChatRequest{Model: "m"})
	if err == nil || errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("a 400 gave error %v, want a request error rather than an outage", err)
	}
}

func TestEmbedRefusesAVectorOfTheWrongWidth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"data":[{"embedding":[0.1,0.2]}]}`)
	}))
	defer server.Close()

	// A 2-wide vector written into a vector(768) column would fail in the
	// database, far from the provider that produced it.
	client := New("openai", server.URL, "sk", time.Second).WithEmbedding("text-embedding-3-small", 768)
	if _, err := client.Embed(context.Background(), []string{"hello"}); err == nil {
		t.Fatal("Embed() accepted a vector of the wrong width")
	}
}

func TestModelsTreatsAMissingCatalogueAsEmpty(t *testing.T) {
	// Several self-hosted OpenAI-compatible gateways have no /models. Refusing to
	// route to them for that would invent a requirement the contract lacks.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := New("vllm", server.URL, "", time.Second)
	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("Models() returned error: %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("Models() = %v, want empty", models)
	}
}
