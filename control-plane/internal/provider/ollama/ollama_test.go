package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return New(server.URL, 5*time.Second)
}

func TestHealthUsesVersionEndpoint(t *testing.T) {
	var path string
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"version":"0.32.13"}`))
	})

	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health() returned error: %v", err)
	}
	// Health must not load a model; /api/version is the cheap reachability probe.
	if path != "/api/version" {
		t.Errorf("Health() called %q, want /api/version", path)
	}
}

func TestHealthReportsUnavailable(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := client.Health(context.Background())
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("Health() error = %v, want ErrUnavailable", err)
	}
}

func TestModelsMapsUpstreamShape(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"model":"qwen2.5:0.5b","size":397821319,
			"details":{"family":"qwen2","parameter_size":"494.03M","quantization_level":"Q4_K_M"}}]}`))
	})

	models, err := client.Models(context.Background())
	if err != nil {
		t.Fatalf("Models() returned error: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("Models() returned %d models, want 1", len(models))
	}
	want := provider.Model{
		ID: "qwen2.5:0.5b", Family: "qwen2",
		ParameterSize: "494.03M", Quantization: "Q4_K_M", SizeBytes: 397821319,
	}
	if models[0] != want {
		t.Errorf("Models()[0] = %+v, want %+v", models[0], want)
	}
}

func TestChatSendsNonStreamingRequestAndMapsUsage(t *testing.T) {
	var received map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"model":"qwen2.5:0.5b","message":{"role":"assistant","content":"hello"},
			"done_reason":"stop","prompt_eval_count":11,"eval_count":4}`))
	})

	temperature, maxTokens := 0.2, 64
	response, err := client.Chat(context.Background(), provider.ChatRequest{
		Model:       "qwen2.5:0.5b",
		Messages:    []provider.Message{{Role: "user", Content: "hi"}},
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}

	// Streaming belongs to the Gateway phase; this adapter must never ask for it.
	if received["stream"] != false {
		t.Errorf("request stream = %v, want false", received["stream"])
	}
	options, ok := received["options"].(map[string]any)
	if !ok {
		t.Fatalf("request has no options object: %v", received)
	}
	if options["temperature"] != 0.2 {
		t.Errorf("options.temperature = %v, want 0.2", options["temperature"])
	}
	if options["num_predict"] != float64(64) {
		t.Errorf("options.num_predict = %v, want 64", options["num_predict"])
	}

	if response.Content != "hello" {
		t.Errorf("Content = %q, want hello", response.Content)
	}
	if response.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", response.FinishReason)
	}
	want := provider.Usage{PromptTokens: 11, CompletionTokens: 4, TotalTokens: 15}
	if response.Usage != want {
		t.Errorf("Usage = %+v, want %+v", response.Usage, want)
	}
}

// Omitting the tuning knobs must not send an empty options object, which would
// override provider defaults with nothing.
func TestChatOmitsOptionsWhenUnset(t *testing.T) {
	var received map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"model":"m","message":{"content":"x"}}`))
	})

	if _, err := client.Chat(context.Background(), provider.ChatRequest{
		Model:    "m",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}); err != nil {
		t.Fatalf("Chat() returned error: %v", err)
	}
	if _, present := received["options"]; present {
		t.Errorf("request sent an options object when none was configured: %v", received)
	}
}

func TestChatStreamEmitsIncrementsAndAggregates(t *testing.T) {
	var received map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		for _, line := range []string{
			`{"model":"qwen2.5:0.5b","message":{"role":"assistant","content":"PO"},"done":false}`,
			`{"model":"qwen2.5:0.5b","message":{"role":"assistant","content":"NG"},"done":false}`,
			`{"model":"qwen2.5:0.5b","message":{"role":"assistant","content":""},"done":true,` +
				`"done_reason":"stop","prompt_eval_count":37,"eval_count":4}`,
		} {
			_, _ = w.Write([]byte(line + "\n"))
		}
	})

	var chunks []string
	response, err := client.ChatStream(context.Background(), provider.ChatRequest{
		Model: "qwen2.5:0.5b", Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}, func(chunk provider.Chunk) error {
		chunks = append(chunks, chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("ChatStream() returned error: %v", err)
	}

	if received["stream"] != true {
		t.Errorf("request stream = %v, want true", received["stream"])
	}
	if len(chunks) != 2 || chunks[0] != "PO" || chunks[1] != "NG" {
		t.Errorf("chunks = %v, want the two content increments only", chunks)
	}
	// The aggregate is what usage accounting is built from, so it must match
	// what the client actually received.
	if response.Content != "PONG" {
		t.Errorf("Content = %q, want the concatenated chunks", response.Content)
	}
	if response.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want stop", response.FinishReason)
	}
	want := provider.Usage{PromptTokens: 37, CompletionTokens: 4, TotalTokens: 41}
	if response.Usage != want {
		t.Errorf("Usage = %+v, want %+v from the final chunk", response.Usage, want)
	}
}

// A stream that ends before the provider says it is done produced a truncated
// answer, which must not be reported as a success.
func TestChatStreamRejectsTruncatedStream(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":{"content":"half"},"done":false}` + "\n"))
	})

	_, err := client.ChatStream(context.Background(), provider.ChatRequest{
		Model: "m", Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}, func(provider.Chunk) error { return nil })

	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

// When the client goes away the adapter must stop rather than drain the rest of
// the upstream response.
func TestChatStreamStopsWhenEmitFails(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		for range 10 {
			_, _ = w.Write([]byte(`{"message":{"content":"x"},"done":false}` + "\n"))
		}
		_, _ = w.Write([]byte(`{"done":true,"done_reason":"stop"}` + "\n"))
	})

	stop := errors.New("client gone")
	emitted := 0
	_, err := client.ChatStream(context.Background(), provider.ChatRequest{
		Model: "m", Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}, func(provider.Chunk) error {
		emitted++
		return stop
	})

	if !errors.Is(err, stop) {
		t.Fatalf("error = %v, want the emit error propagated", err)
	}
	if emitted != 1 {
		t.Errorf("emitted %d chunks after a failure, want 1", emitted)
	}
}

func TestChatStreamReportsUnavailable(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	_, err := client.ChatStream(context.Background(), provider.ChatRequest{
		Model: "m", Messages: []provider.Message{{Role: "user", Content: "hi"}},
	}, func(provider.Chunk) error { return nil })

	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}

func TestChatReportsUnavailable(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"model not found"}`))
	})

	_, err := client.Chat(context.Background(), provider.ChatRequest{
		Model:    "missing",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})
	if !errors.Is(err, provider.ErrUnavailable) {
		t.Fatalf("Chat() error = %v, want ErrUnavailable", err)
	}
}
