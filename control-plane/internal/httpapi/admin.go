package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
)

// KeyAdmin manages API key records.
type KeyAdmin interface {
	Create(ctx context.Context, params auth.CreateParams) (auth.Record, string, error)
	List(ctx context.Context) ([]auth.Record, error)
	Revoke(ctx context.Context, id string) (auth.Record, error)
}

type createKeyBody struct {
	Name               string   `json:"name"`
	Scopes             []string `json:"scopes"`
	CompanyID          string   `json:"companyId,omitempty"`
	Department         string   `json:"department,omitempty"`
	MaxClassification  string   `json:"maxClassification,omitempty"`
	RateLimitPerMinute int      `json:"rateLimitPerMinute,omitempty"`
	ExpiresAt          *string  `json:"expiresAt,omitempty"`
}

// registerAdmin adds the API key management surface. Every route requires the
// admin:keys scope, so a key can only mint further keys if it was explicitly
// granted that power.
func registerAdmin(mux *http.ServeMux, d Deps) {
	if d.Keys == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("POST /api/v1/admin/api-keys", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body createKeyBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		var expiresAt *time.Time
		if body.ExpiresAt != nil {
			parsed, err := time.Parse(time.RFC3339, *body.ExpiresAt)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expiresAt must be an RFC3339 timestamp"})
				return
			}
			expiresAt = &parsed
		}
		if body.RateLimitPerMinute == 0 {
			body.RateLimitPerMinute = d.Config.DefaultRateLimitPerMinute
		}

		// A company-scoped key cannot mint one for another company.
		companyID := body.CompanyID
		if caller.CompanyID != "" {
			if companyID != "" && companyID != caller.CompanyID {
				d.recordDenied(r, caller, auth.ScopeAdminKeys, "company mismatch")
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "cannot create a key for another company",
				})
				return
			}
			companyID = caller.CompanyID
		}

		// A key cannot mint one that reads above its own clearance: that would
		// make admin:keys a privilege-escalation path.
		clearance := body.MaxClassification
		if clearance != "" && !d.Knowledge.Policy.Allows(caller.MaxClassification, clearance) {
			d.recordDenied(r, caller, auth.ScopeAdminKeys, "clearance above own")
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "cannot grant a clearance above this key's own",
			})
			return
		}

		record, secret, err := d.Keys.Create(r.Context(), auth.CreateParams{
			Name:               body.Name,
			Scopes:             body.Scopes,
			CompanyID:          companyID,
			Department:         body.Department,
			MaxClassification:  clearance,
			RateLimitPerMinute: body.RateLimitPerMinute,
			CreatedBy:          caller.KeyID,
			ExpiresAt:          expiresAt,
		})
		if err != nil {
			// Scope typos and missing names are the caller's mistake, not a fault.
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "api_key.created", ResourceType: "api_key", ResourceID: record.ID,
			Metadata: map[string]any{"name": record.Name, "scopes": record.Scopes},
		})

		// The raw value is shown once only and is never stored or logged.
		writeJSON(w, http.StatusCreated, map[string]any{
			"apiKey": record,
			"secret": secret,
			"notice": "Store this value now. It cannot be retrieved again.",
		})
	}))

	mux.HandleFunc("GET /api/v1/admin/api-keys", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		records, err := d.Keys.List(r.Context())
		if err != nil {
			d.Log.Error("list api keys", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if records == nil {
			records = []auth.Record{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"apiKeys": records})
	}))

	// The outbox has no publisher until Kafka is deployed, so its backlog needs
	// to be visible somewhere.
	mux.HandleFunc("GET /api/v1/admin/outbox", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		auditRows, usageRows, err := d.Audit.PendingCounts(r.Context())
		if err != nil {
			d.Log.Error("read outbox backlog", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"pending": map[string]int{"auditLogs": auditRows, "usageEvents": usageRows},
			"topics":  map[string]string{"auditLogs": "ai.audit.v1", "usageEvents": "ai.usage.v1"},
			"note":    "Rows stay unpublished until a Kafka publisher is deployed.",
		})
	}))

	mux.HandleFunc("POST /api/v1/admin/api-keys/{id}/revoke", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		id := r.PathValue("id")

		record, err := d.Keys.Revoke(r.Context(), id)
		switch {
		case errors.Is(err, auth.ErrKeyNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "api key not found"})
			return
		case err != nil:
			d.Log.Error("revoke api key", "keyId", id, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "api_key.revoked", ResourceType: "api_key", ResourceID: record.ID,
			Metadata: map[string]any{"name": record.Name},
		})
		writeJSON(w, http.StatusOK, map[string]any{"apiKey": record})
	}))
}
