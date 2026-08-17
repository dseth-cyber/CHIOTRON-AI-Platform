package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

type streamingProvider struct {
	fakeProvider
	chunks   []string
	streamed bool
}

func (s *streamingProvider) ChatStream(_ context.Context, req provider.ChatRequest, emit func(provider.Chunk) error) (provider.ChatResponse, error) {
	s.streamed = true
	if s.chatErr != nil {
		return provider.ChatResponse{}, s.chatErr
	}
	content := ""
	for _, chunk := range s.chunks {
		if err := emit(provider.Chunk{Content: chunk}); err != nil {
			return provider.ChatResponse{}, err
		}
		content += chunk
	}
	return provider.ChatResponse{
		Model: req.Model, Content: content, FinishReason: "stop",
		Usage:     provider.Usage{PromptTokens: 37, CompletionTokens: 4, TotalTokens: 41},
		LatencyMs: 12,
	}, nil
}

func streamingFixture(t *testing.T, chunks ...string) (computeFixture, *streamingProvider) {
	t.Helper()
	p := &streamingProvider{
		fakeProvider: fakeProvider{name: "ollama", models: []provider.Model{{ID: "qwen2.5:0.5b"}}},
		chunks:       chunks,
	}
	return newComputeFixture(t, p), p
}

// sseFrames splits an event-stream body into its data payloads.
func sseFrames(t *testing.T, body string) []string {
	t.Helper()
	var frames []string
	for _, block := range strings.Split(body, "\n\n") {
		for _, line := range strings.Split(block, "\n") {
			if payload, ok := strings.CutPrefix(line, "data: "); ok {
				frames = append(frames, payload)
			}
		}
	}
	return frames
}

func TestStreamChatEmitsEventStream(t *testing.T) {
	fixture, p := streamingFixture(t, "PO", "NG")

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}],"stream":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if !p.streamed {
		t.Error("the streaming provider was not used")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", got)
	}
	// Nginx buffers proxied responses by default, which would defeat streaming.
	if got := rec.Header().Get("X-Accel-Buffering"); got != "no" {
		t.Errorf("X-Accel-Buffering = %q, want no", got)
	}

	frames := sseFrames(t, rec.Body.String())
	if len(frames) != 4 {
		t.Fatalf("got %d frames, want two content frames, a done frame and the terminator: %v", len(frames), frames)
	}
	if frames[len(frames)-1] != sseTerminator {
		t.Errorf("last frame = %q, want %q", frames[len(frames)-1], sseTerminator)
	}

	var first provider.Chunk
	if err := json.Unmarshal([]byte(frames[0]), &first); err != nil {
		t.Fatalf("first frame is not JSON: %v", err)
	}
	if first.Content != "PO" {
		t.Errorf("first chunk content = %q, want PO", first.Content)
	}

	// Usage arrives on the final frame, which is the only place the provider
	// knows it.
	var final provider.Chunk
	if err := json.Unmarshal([]byte(frames[2]), &final); err != nil {
		t.Fatalf("done frame is not JSON: %v", err)
	}
	if !final.Done || final.FinishReason != "stop" {
		t.Errorf("done frame = %+v, want done with a finish reason", final)
	}
	if final.Usage == nil || final.Usage.TotalTokens != 41 {
		t.Errorf("done frame usage = %+v, want the provider's counts", final.Usage)
	}
}

// Streaming must account for tokens exactly like the non-streaming path.
func TestStreamChatRecordsUsage(t *testing.T) {
	fixture, _ := streamingFixture(t, "PO", "NG")

	authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}],"stream":true}`)

	if len(fixture.audit.usage) != 1 {
		t.Fatalf("recorded %d usage events, want 1", len(fixture.audit.usage))
	}
	usage := fixture.audit.usage[0]
	if usage.TotalTokens != 41 || usage.Outcome != audit.OutcomeSuccess {
		t.Errorf("usage = %+v, want the provider counts and a success outcome", usage)
	}
}

// Until a chunk has been written the real status can still be reported, so a
// dead compute plane is a 503 rather than an error buried inside a 200 stream.
func TestStreamChatFailsBeforeFirstChunkWithRealStatus(t *testing.T) {
	down := &streamingProvider{fakeProvider: fakeProvider{name: "ollama", chatErr: provider.ErrUnavailable}}
	fixture := newComputeFixture(t, down)

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}],"stream":true}`)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); strings.Contains(got, "event-stream") {
		t.Errorf("Content-Type = %q, want a JSON error rather than a stream", got)
	}
	if len(fixture.audit.usage) != 1 || fixture.audit.usage[0].Outcome != audit.OutcomeFailure {
		t.Errorf("usage = %+v, want one failed call recorded", fixture.audit.usage)
	}
}

// An unknown model is rejected before anything is written, in both modes.
func TestStreamChatRejectsUnknownModelWith404(t *testing.T) {
	fixture, _ := streamingFixture(t, "x")

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"model":"nope","messages":[{"role":"user","content":"hi"}],"stream":true}`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// A caller that asks for a stream gets one even when the backend cannot stream.
func TestStreamChatFallsBackToWholeResponse(t *testing.T) {
	fixture := newComputeFixture(t, availableProvider())

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}],"stream":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	frames := sseFrames(t, rec.Body.String())
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want one content frame, a done frame and the terminator: %v", len(frames), frames)
	}

	var first provider.Chunk
	if err := json.Unmarshal([]byte(frames[0]), &first); err != nil {
		t.Fatalf("first frame is not JSON: %v", err)
	}
	if first.Content != "hello" {
		t.Errorf("content = %q, want the whole response in one chunk", first.Content)
	}
}

// The non-streaming path must stay JSON.
func TestChatWithoutStreamFlagStaysJSON(t *testing.T) {
	fixture, p := streamingFixture(t, "PO", "NG")

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}]}`)

	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", got)
	}
	if p.streamed {
		t.Error("the streaming path was used without stream:true")
	}
}

// http.NewResponseController must be able to reach the real ResponseWriter
// through the middleware chain, or the whole stream would be buffered until the
// handler returned.
func TestResponseControllerReachesWriterThroughMiddleware(t *testing.T) {
	var flushed bool
	handler := chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("Flush() through the middleware chain returned %v", err)
			return
		}
		flushed = true
	}), recoverPanic(quietLogger()), requestLog(quietLogger()), securityHeaders)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/stream", nil))
	if !flushed {
		t.Error("the handler could not flush through the middleware chain")
	}
}
