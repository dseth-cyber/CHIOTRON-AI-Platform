package httpapi

import (
	"net/http"
	"time"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

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

// registerCompute adds the compute-plane routes.
//
// None of these feed /readyz. Losing VM5 degrades model calls only; the
// Control Plane stays ready and reports the provider as unavailable
// (ARCHITECTURE-v1 section 9).
func registerCompute(mux *http.ServeMux, d Deps) {
	if d.Compute == nil || !d.authenticated() {
		return
	}
	cfg := d.Config

	mux.HandleFunc("GET /api/v1/compute/health", d.guard(auth.ScopeModelsRead, func(w http.ResponseWriter, r *http.Request) {
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
	}))

	mux.HandleFunc("GET /api/v1/models", d.guard(auth.ScopeModelsRead, func(w http.ResponseWriter, r *http.Request) {
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
	}))
}
