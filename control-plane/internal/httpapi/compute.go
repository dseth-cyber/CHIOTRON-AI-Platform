package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// maxChatBody bounds an inbound prompt. The Gateway phase replaces this with
// per-identity quota; until then it is the only backstop.
const maxChatBody = 1 << 20

type providerHealth struct {
	Status    string           `json:"status"`
	LatencyMs int64            `json:"latencyMs"`
	Error     string           `json:"error,omitempty"`
	Models    []provider.Model `json:"models,omitempty"`
}

type logicalModel struct {
	Logical   string `json:"logical"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Available bool   `json:"available"`
	Default   bool   `json:"default,omitempty"`
}

type chatRequestBody struct {
	Model       string             `json:"model"`
	Messages    []provider.Message `json:"messages"`
	Temperature *float64           `json:"temperature,omitempty"`
	MaxTokens   *int               `json:"maxTokens,omitempty"`
}

// registerCompute adds the compute-plane routes.
//
// None of these feed /readyz. Losing VM5 degrades model calls only; the
// Control Plane stays ready and reports the provider as unavailable
// (ARCHITECTURE-v1 section 9).
func registerCompute(mux *http.ServeMux, d Deps) {
	if d.Compute == nil {
		return
	}
	cfg := d.Config

	mux.HandleFunc("GET /api/v1/compute/health", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextWithTimeout(r, cfg.ComputeHealthTimeout)
		defer cancel()

		providers := make(map[string]providerHealth)
		available, total := 0, 0
		for _, llm := range d.Compute.Providers() {
			total++
			started := time.Now()
			health := providerHealth{Status: "ok"}
			if err := llm.Health(ctx); err != nil {
				health.Status = "unavailable"
				health.Error = err.Error()
			} else {
				available++
				if models, err := llm.Models(ctx); err == nil {
					health.Models = models
				}
			}
			health.LatencyMs = time.Since(started).Milliseconds()
			providers[llm.Name()] = health
		}

		status := "unavailable"
		switch {
		case total == 0 || available == total:
			status = "available"
		case available > 0:
			status = "degraded"
		}

		// Always 200: this endpoint reports on the compute plane, it does not
		// fail with it.
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    status,
			"providers": providers,
			"time":      time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("GET /api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := contextWithTimeout(r, cfg.ComputeHealthTimeout)
		defer cancel()

		// Ask each provider once, however many routes point at it.
		loaded := make(map[string]map[string]bool)
		for _, llm := range d.Compute.Providers() {
			present := make(map[string]bool)
			if models, err := llm.Models(ctx); err == nil {
				for _, model := range models {
					present[model.ID] = true
				}
			}
			loaded[llm.Name()] = present
		}

		routes := d.Compute.Routes()
		models := make([]logicalModel, 0, len(routes))
		for _, route := range routes {
			models = append(models, logicalModel{
				Logical:   route.Logical,
				Provider:  route.Provider,
				Model:     route.Model,
				Available: loaded[route.Provider][route.Model],
				Default:   route.Logical == d.Compute.DefaultModel(),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"default": d.Compute.DefaultModel(),
			"models":  models,
		})
	})

	if !cfg.DevUnauthenticatedChat {
		return
	}
	mux.HandleFunc("POST /api/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		var body chatRequestBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChatBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if len(body.Messages) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "messages must not be empty"})
			return
		}

		ctx, cancel := contextWithTimeout(r, cfg.ComputeTimeout)
		defer cancel()

		response, route, err := d.Compute.Chat(ctx, body.Model, provider.ChatRequest{
			Messages:    body.Messages,
			Temperature: body.Temperature,
			MaxTokens:   body.MaxTokens,
		})
		switch {
		case errors.Is(err, provider.ErrUnknownModel):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		case errors.Is(err, provider.ErrUnavailable):
			// The Control Plane is fine; the compute plane is not.
			d.Log.Error("compute call failed", "provider", route.Provider, "model", route.Model, "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "compute provider unavailable"})
			return
		case err != nil:
			d.Log.Error("compute call failed", "error", err)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "compute call failed"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"logicalModel": route.Logical,
			"provider":     route.Provider,
			"model":        response.Model,
			"content":      response.Content,
			"finishReason": response.FinishReason,
			"usage":        response.Usage,
			"latencyMs":    response.LatencyMs,
		})
	})
}
