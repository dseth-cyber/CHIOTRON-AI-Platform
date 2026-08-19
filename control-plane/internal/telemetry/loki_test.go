package telemetry

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestLokiHandlerStreamFormatAndPush(t *testing.T) {
	var mu sync.Mutex
	var receivedPayloads []lokiPayload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		if r.URL.Path != "/loki/api/v1/push" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Scope-OrgID") != "test-tenant" {
			t.Errorf("Unexpected tenant header: %s", r.Header.Get("X-Scope-OrgID"))
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var p lokiPayload
		if err := json.Unmarshal(body, &p); err != nil {
			t.Errorf("Unmarshal loki payload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		receivedPayloads = append(receivedPayloads, p)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	handler := NewLokiHandler(LokiConfig{
		URL:           ts.URL,
		TenantID:      "test-tenant",
		ServiceName:   "ai-control-plane",
		Environment:   "test",
		BatchSize:     2,
		FlushInterval: 100 * time.Millisecond,
	})
	defer handler.Close()

	logger := slog.New(handler)
	ctx := context.Background()

	logger.InfoContext(ctx, "First test message", "key1", "val1")
	logger.WarnContext(ctx, "Second test message", "key2", "val2")

	// Trigger manual flush
	handler.Flush(ctx)

	mu.Lock()
	count := len(receivedPayloads)
	mu.Unlock()

	if count == 0 {
		t.Fatal("Expected at least 1 push to Loki server, got 0")
	}

	mu.Lock()
	firstStream := receivedPayloads[0].Streams[0]
	mu.Unlock()

	if firstStream.Stream["service_name"] != "ai-control-plane" {
		t.Errorf("Stream label service_name = %s", firstStream.Stream["service_name"])
	}
	if firstStream.Stream["level"] != "INFO" {
		t.Errorf("Stream label level = %s", firstStream.Stream["level"])
	}
}
