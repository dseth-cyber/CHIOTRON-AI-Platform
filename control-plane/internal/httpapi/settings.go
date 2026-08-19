package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/settings"
)

type SettingsAdmin interface {
	List(ctx context.Context) ([]settings.Setting, error)
	Get(ctx context.Context, key string) (settings.Setting, error)
	Set(ctx context.Context, key string, jsonValue string, description string, updatedBy string) (settings.Setting, error)
}

type updateSettingBody struct {
	Value       any    `json:"value"`
	Description string `json:"description,omitempty"`
}

func registerSettings(mux *http.ServeMux, d Deps) {
	if d.Settings == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("GET /api/v1/admin/settings", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		items, err := d.Settings.List(r.Context())
		if err != nil {
			d.Log.Error("list platform settings", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list settings"})
			return
		}
		writeJSON(w, http.StatusOK, items)
	}))

	mux.HandleFunc("PUT /api/v1/admin/settings/{key}", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		key := r.PathValue("key")
		if key == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "setting key is required"})
			return
		}

		var body updateSettingBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		rawJSON, err := json.Marshal(body.Value)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "value must be valid JSON"})
			return
		}

		item, err := d.Settings.Set(r.Context(), key, string(rawJSON), body.Description, caller.KeyID)
		if err != nil {
			d.Log.Error("update platform setting", "key", key, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update setting"})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID:      caller.KeyID,
			APIKeyID:     caller.KeyID,
			CompanyID:    caller.CompanyID,
			Action:       "settings.update",
			ResourceType: "setting",
			ResourceID:   key,
			Metadata: map[string]any{
				"key": key,
			},
		})

		writeJSON(w, http.StatusOK, item)
	}))
}
