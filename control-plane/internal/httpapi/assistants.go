package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
)

// AssistantCatalogue is the assistant registry the Gateway reads from.
type AssistantCatalogue interface {
	List(ctx context.Context, companyID string) ([]assistant.Assistant, error)
	Resolve(ctx context.Context, reference, companyID string) (assistant.Assistant, error)
	Create(ctx context.Context, params assistant.CreateParams) (assistant.Assistant, error)
}

type createAssistantBody struct {
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	Instructions string   `json:"instructions,omitempty"`
	LogicalModel string   `json:"logicalModel"`
	Temperature  *float64 `json:"temperature,omitempty"`
	MaxTokens    *int     `json:"maxTokens,omitempty"`
	CompanyID    string   `json:"companyId,omitempty"`
}

func registerAssistants(mux *http.ServeMux, d Deps) {
	if d.Assistants == nil || !d.authenticated() {
		return
	}

	// The catalogue is filtered by the caller's company in SQL, so another
	// company's assistant never reaches this handler.
	mux.HandleFunc("GET /api/v1/assistants", d.guard(auth.ScopeAssistantsRead, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		records, err := d.Assistants.List(r.Context(), caller.CompanyID)
		if err != nil {
			d.Log.Error("list assistants", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// Instructions are assistant policy; the catalogue does not hand them out.
		public := make([]assistant.Assistant, 0, len(records))
		for _, record := range records {
			public = append(public, record.Public())
		}
		writeJSON(w, http.StatusOK, map[string]any{"assistants": public})
	}))

	mux.HandleFunc("POST /api/v1/admin/assistants", d.guard(auth.ScopeAdminAssistants, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body createAssistantBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		// A key scoped to one company cannot create an assistant for another.
		companyID := body.CompanyID
		if caller.CompanyID != "" {
			if companyID != "" && companyID != caller.CompanyID {
				d.Log.Warn("assistant company mismatch", "keyId", caller.KeyID, "requested", companyID)
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot create an assistant for another company"})
				return
			}
			companyID = caller.CompanyID
		}

		record, err := d.Assistants.Create(r.Context(), assistant.CreateParams{
			Slug:         body.Slug,
			Name:         body.Name,
			Description:  body.Description,
			Instructions: body.Instructions,
			LogicalModel: body.LogicalModel,
			Temperature:  body.Temperature,
			MaxTokens:    body.MaxTokens,
			CompanyID:    companyID,
			CreatedBy:    caller.KeyID,
		})
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "assistant.created", ResourceType: "assistant", ResourceID: record.ID,
			Metadata: map[string]any{"slug": record.Slug, "logicalModel": record.LogicalModel},
		})
		writeJSON(w, http.StatusCreated, map[string]any{"assistant": record})
	}))
}
