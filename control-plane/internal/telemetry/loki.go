// Package telemetry: Loki Log Shipper and Structured Log Handler.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// LokiConfig specifies options for shipping logs to Grafana Loki.
type LokiConfig struct {
	URL           string
	TenantID      string
	ServiceName   string
	Environment   string
	BatchSize     int
	FlushInterval time.Duration
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

type lokiPayload struct {
	Streams []lokiStream `json:"streams"`
}

// LokiHandler is an slog.Handler that formats and ships log entries to Grafana Loki.
type LokiHandler struct {
	cfg     LokiConfig
	client  *http.Client
	mu      sync.Mutex
	buffer  []lokiStream
	attrs   []slog.Attr
	groups  []string
	done    chan struct{}
	stopped bool
}

// NewLokiHandler creates a new asynchronous Loki log shipping handler.
func NewLokiHandler(cfg LokiConfig) *LokiHandler {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "ai-control-plane"
	}
	if cfg.Environment == "" {
		cfg.Environment = "development"
	}

	h := &LokiHandler{
		cfg:    cfg,
		client: &http.Client{Timeout: 5 * time.Second},
		buffer: make([]lokiStream, 0, cfg.BatchSize),
		done:   make(chan struct{}),
	}

	go h.flushLoop()
	return h
}

func (h *LokiHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return true
}

func (h *LokiHandler) Handle(ctx context.Context, r slog.Record) error {
	fields := make(map[string]any)
	fields["msg"] = r.Message
	fields["time"] = r.Time.UTC().Format(time.RFC3339Nano)
	fields["level"] = r.Level.String()

	// Capture OpenTelemetry Trace ID and Span ID if present
	span := trace.SpanFromContext(ctx)
	if span != nil && span.SpanContext().IsValid() {
		fields["trace_id"] = span.SpanContext().TraceID().String()
		fields["span_id"] = span.SpanContext().SpanID().String()
	}

	// Capture attrs
	for _, attr := range h.attrs {
		fields[attr.Key] = attr.Value.Any()
	}
	r.Attrs(func(attr slog.Attr) bool {
		fields[attr.Key] = attr.Value.Any()
		return true
	})

	rawJSON, err := json.Marshal(fields)
	if err != nil {
		return err
	}

	timestampNs := fmt.Sprintf("%d", r.Time.UTC().UnixNano())
	labels := map[string]string{
		"service_name": h.cfg.ServiceName,
		"environment":  h.cfg.Environment,
		"level":        r.Level.String(),
	}

	stream := lokiStream{
		Stream: labels,
		Values: [][]string{{timestampNs, string(rawJSON)}},
	}

	h.mu.Lock()
	h.buffer = append(h.buffer, stream)
	shouldFlush := len(h.buffer) >= h.cfg.BatchSize
	h.mu.Unlock()

	if shouldFlush {
		h.Flush(ctx)
	}

	return nil
}

func (h *LokiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	return &LokiHandler{
		cfg:    h.cfg,
		client: h.client,
		attrs:  append(append([]slog.Attr{}, h.attrs...), attrs...),
		groups: h.groups,
		done:   h.done,
	}
}

func (h *LokiHandler) WithGroup(name string) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	return &LokiHandler{
		cfg:    h.cfg,
		client: h.client,
		attrs:  h.attrs,
		groups: append(append([]string{}, h.groups...), name),
		done:   h.done,
	}
}

func (h *LokiHandler) flushLoop() {
	ticker := time.NewTicker(h.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.Flush(context.Background())
		case <-h.done:
			h.Flush(context.Background())
			return
		}
	}
}

// Flush pushes any queued log streams to the Loki endpoint.
func (h *LokiHandler) Flush(ctx context.Context) {
	h.mu.Lock()
	if len(h.buffer) == 0 {
		h.mu.Unlock()
		return
	}
	batch := h.buffer
	h.buffer = make([]lokiStream, 0, h.cfg.BatchSize)
	h.mu.Unlock()

	if h.cfg.URL == "" {
		return // No endpoint configured
	}

	payload := lokiPayload{Streams: batch}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	endpoint := strings.TrimRight(h.cfg.URL, "/")
	if !strings.HasSuffix(endpoint, "/loki/api/v1/push") {
		endpoint = endpoint + "/loki/api/v1/push"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if h.cfg.TenantID != "" {
		req.Header.Set("X-Scope-OrgID", h.cfg.TenantID)
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

// Close gracefully flushes pending logs and stops the background loop.
func (h *LokiHandler) Close() error {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		return nil
	}
	h.stopped = true
	close(h.done)
	h.mu.Unlock()
	return nil
}
