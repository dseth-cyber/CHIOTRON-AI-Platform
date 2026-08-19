package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/prompt"
)

type PromptAdmin interface {
	List(ctx context.Context) ([]prompt.Template, error)
	GetBySlug(ctx context.Context, slug string) (prompt.Template, error)
	Create(ctx context.Context, params prompt.CreateParams) (prompt.Template, error)
	Delete(ctx context.Context, id string) error
}

type createPromptBody struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Template    string   `json:"template"`
	Variables   []string `json:"variables,omitempty"`
}

func registerPrompts(mux *http.ServeMux, d Deps) {
	if d.Prompts == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("GET /api/v1/prompts", d.guard(anyScope, func(w http.ResponseWriter, r *http.Request) {
		templates, err := d.Prompts.List(r.Context())
		if err != nil {
			d.Log.Error("list prompt templates", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list prompt templates"})
			return
		}
		writeJSON(w, http.StatusOK, templates)
	}))

	mux.HandleFunc("POST /api/v1/admin/prompts", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body createPromptBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		tpl, err := d.Prompts.Create(r.Context(), prompt.CreateParams{
			Slug:        body.Slug,
			Name:        body.Name,
			Description: body.Description,
			Template:    body.Template,
			Variables:   body.Variables,
			CreatedBy:   caller.KeyID,
		})
		if err != nil {
			d.Log.Error("create prompt template", "slug", body.Slug, "error", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID:      caller.KeyID,
			APIKeyID:     caller.KeyID,
			CompanyID:    caller.CompanyID,
			Action:       "prompt.create",
			ResourceType: "prompt_template",
			ResourceID:   tpl.ID,
			Metadata: map[string]any{
				"slug": tpl.Slug,
				"id":   tpl.ID,
			},
		})

		writeJSON(w, http.StatusCreated, tpl)
	}))

	mux.HandleFunc("DELETE /api/v1/admin/prompts/{id}", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		id := r.PathValue("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
			return
		}

		if err := d.Prompts.Delete(r.Context(), id); err != nil {
			if errors.Is(err, prompt.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "prompt template not found"})
				return
			}
			d.Log.Error("delete prompt template", "id", id, "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete prompt template"})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID:      caller.KeyID,
			APIKeyID:     caller.KeyID,
			CompanyID:    caller.CompanyID,
			Action:       "prompt.delete",
			ResourceType: "prompt_template",
			ResourceID:   id,
			Metadata: map[string]any{
				"id": id,
			},
		})

		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}))
}
