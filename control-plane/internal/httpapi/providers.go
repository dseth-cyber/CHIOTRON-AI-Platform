package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/compute"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/secret"
)

// ProviderAdmin is the provider registry as the HTTP layer needs it.
type ProviderAdmin interface {
	ListProviders(ctx context.Context) ([]compute.Provider, error)
	CreateProvider(ctx context.Context, params compute.CreateParams) (compute.Provider, error)
	UpdateProvider(ctx context.Context, slug string, params compute.UpdateParams) (compute.Provider, error)
	DeleteProvider(ctx context.Context, slug string) error
	RecordCheck(ctx context.Context, slug, status, failure string) error
	Adapter(ctx context.Context, record compute.Provider) (provider.LLM, error)
	ListRoutes(ctx context.Context) ([]compute.Route, error)
	SaveRoute(ctx context.Context, params compute.RouteParams) (compute.Route, error)
	DeleteRoute(ctx context.Context, logical string) error
}

// Reloader rebuilds the live routing table after a change.
//
// Configuration that needs a restart to take effect is configuration an
// operator will not touch in production (ARCHITECTURE-v1 section 46), so every
// write here is followed by a reload.
type Reloader interface {
	Reload(ctx context.Context) error
}

type providerBody struct {
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	Description       string `json:"description,omitempty"`
	Kind              string `json:"kind"`
	BaseURL           string `json:"baseUrl"`
	Credential        string `json:"credential,omitempty"`
	MaxClassification string `json:"maxClassification,omitempty"`
	TimeoutSeconds    int    `json:"timeoutSeconds,omitempty"`
	CompanyID         string `json:"companyId,omitempty"`
}

type providerPatch struct {
	Name              *string `json:"name,omitempty"`
	Description       *string `json:"description,omitempty"`
	BaseURL           *string `json:"baseUrl,omitempty"`
	Credential        *string `json:"credential,omitempty"`
	MaxClassification *string `json:"maxClassification,omitempty"`
	Enabled           *bool   `json:"enabled,omitempty"`
	TimeoutSeconds    *int    `json:"timeoutSeconds,omitempty"`
}

type routeBody struct {
	Logical   string `json:"logical"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	IsDefault bool   `json:"default,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
	CompanyID string `json:"companyId,omitempty"`
}

// registerProviders adds the model-routing admin surface.
//
// It is gated by admin:keys rather than a scope of its own: deciding where the
// platform sends prompts is the same class of decision as minting credentials,
// and both are things a browser should rarely be holding.
func registerProviders(mux *http.ServeMux, d Deps) {
	if d.Providers == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("GET /api/v1/admin/providers", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		providers, err := d.Providers.ListProviders(r.Context())
		if err != nil {
			d.writeProviderError(w, err, "list providers")
			return
		}
		if providers == nil {
			providers = []compute.Provider{}
		}
		routes, err := d.Providers.ListRoutes(r.Context())
		if err != nil {
			d.writeProviderError(w, err, "list routes")
			return
		}
		if routes == nil {
			routes = []compute.Route{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"providers": providers,
			"routes":    routes,
			// The client needs these to build its forms without hard-coding a list
			// that would drift from the adapters that actually exist.
			"kinds":           compute.Kinds,
			"classifications": d.Knowledge.Policy.Levels(),
			// Whether a credential can be stored at all, so the UI can explain a
			// refusal before somebody types a key into a form that will reject it.
			"credentialStorage": d.CredentialStorage,
		})
	}))

	mux.HandleFunc("POST /api/v1/admin/providers", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body providerBody
		if !decodeJSON(w, r, &body) {
			return
		}
		classification := body.MaxClassification
		if classification == "" {
			classification = d.Knowledge.Policy.Least()
		}
		if _, err := d.Knowledge.Policy.Normalise(classification); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		record, err := d.Providers.CreateProvider(r.Context(), compute.CreateParams{
			Slug: body.Slug, Name: body.Name, Description: body.Description,
			Kind: body.Kind, BaseURL: body.BaseURL, Credential: body.Credential,
			MaxClassification: classification, TimeoutSeconds: body.TimeoutSeconds,
			CompanyID: body.CompanyID, CreatedBy: caller.KeyID,
		})
		if err != nil {
			d.writeProviderError(w, err, "create provider")
			return
		}

		d.reload(r, caller, "provider.created", record.Slug, map[string]any{
			"kind": record.Kind, "maxClassification": record.MaxClassification,
			// The credential itself is never recorded, only that one was supplied.
			"hasCredential": record.HasCredential,
		})
		writeJSON(w, http.StatusCreated, map[string]any{"provider": record})
	}))

	mux.HandleFunc("PATCH /api/v1/admin/providers/{slug}", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body providerPatch
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.MaxClassification != nil {
			if _, err := d.Knowledge.Policy.Normalise(*body.MaxClassification); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
		}

		record, err := d.Providers.UpdateProvider(r.Context(), r.PathValue("slug"), compute.UpdateParams{
			Name: body.Name, Description: body.Description, BaseURL: body.BaseURL,
			Credential: body.Credential, MaxClassification: body.MaxClassification,
			Enabled: body.Enabled, TimeoutSeconds: body.TimeoutSeconds,
		})
		if err != nil {
			d.writeProviderError(w, err, "update provider")
			return
		}

		// Raising a ceiling is the change worth being able to find later, so it is
		// named in the audit metadata rather than left as "updated".
		d.reload(r, caller, "provider.updated", record.Slug, map[string]any{
			"maxClassification": record.MaxClassification, "enabled": record.Enabled,
			"credentialReplaced": body.Credential != nil,
		})
		writeJSON(w, http.StatusOK, map[string]any{"provider": record})
	}))

	mux.HandleFunc("DELETE /api/v1/admin/providers/{slug}", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		slug := r.PathValue("slug")

		if err := d.Providers.DeleteProvider(r.Context(), slug); err != nil {
			d.writeProviderError(w, err, "delete provider")
			return
		}
		d.reload(r, caller, "provider.deleted", slug, nil)
		w.WriteHeader(http.StatusNoContent)
	}))

	// A connection test reaches the provider directly rather than through the
	// live registry, so a credential can be checked before any traffic is routed
	// to it — and so a provider that is failing can be diagnosed without first
	// being taken out of service.
	mux.HandleFunc("POST /api/v1/admin/providers/{slug}/check", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")

		providers, err := d.Providers.ListProviders(r.Context())
		if err != nil {
			d.writeProviderError(w, err, "list providers")
			return
		}
		index := -1
		for position, record := range providers {
			if record.Slug == slug {
				index = position
			}
		}
		if index < 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provider not found"})
			return
		}

		ctx, cancel := contextWithTimeout(r, d.Config.ComputeHealthTimeout)
		defer cancel()

		status, failure := "available", ""
		adapter, err := d.Providers.Adapter(ctx, providers[index])
		if err == nil {
			err = adapter.Health(ctx)
		}
		if err != nil {
			status, failure = "unavailable", err.Error()
		}
		if err := d.Providers.RecordCheck(r.Context(), slug, status, failure); err != nil {
			d.Log.Error("record provider check", "provider", slug, "error", err)
		}

		var models []provider.Model
		if status == "available" {
			models, _ = adapter.Models(ctx)
		}
		// 200 either way: the check itself succeeded, and what it found is the
		// body. A 503 here would say the Control Plane is unwell, which it is not.
		writeJSON(w, http.StatusOK, map[string]any{
			"status": status, "error": failure, "models": models,
		})
	}))

	mux.HandleFunc("PUT /api/v1/admin/routes", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body routeBody
		if !decodeJSON(w, r, &body) {
			return
		}
		record, err := d.Providers.SaveRoute(r.Context(), compute.RouteParams{
			Logical: body.Logical, ProviderSlug: body.Provider, UpstreamModel: body.Model,
			IsDefault: body.IsDefault, Enabled: body.Enabled, CompanyID: body.CompanyID, CreatedBy: caller.KeyID,
		})
		if err != nil {
			d.writeProviderError(w, err, "save route")
			return
		}
		d.reload(r, caller, "route.saved", record.Logical, map[string]any{
			"provider": record.ProviderSlug, "model": record.UpstreamModel,
			"default": record.IsDefault, "enabled": record.Enabled,
		})
		writeJSON(w, http.StatusOK, map[string]any{"route": record})
	}))

	mux.HandleFunc("DELETE /api/v1/admin/routes/{logical}", d.guard(auth.ScopeAdminKeys, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		logical := r.PathValue("logical")

		if err := d.Providers.DeleteRoute(r.Context(), logical); err != nil {
			d.writeProviderError(w, err, "delete route")
			return
		}
		d.reload(r, caller, "route.deleted", logical, nil)
		w.WriteHeader(http.StatusNoContent)
	}))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return false
	}
	return true
}

// reload applies a routing change to the running process and records it.
//
// A failed reload is logged rather than returned: the change is committed, and
// telling the operator their edit failed when it is in the database would send
// them to make it twice.
func (d Deps) reload(r *http.Request, caller auth.Identity, action, resource string, metadata map[string]any) {
	if d.ReloadCompute != nil {
		if err := d.ReloadCompute.Reload(r.Context()); err != nil {
			d.Log.Error("reload compute registry", "error", err)
		}
	}
	d.Audit.Record(r.Context(), audit.Event{
		ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
		Action: action, ResourceType: "provider", ResourceID: resource,
		Metadata: metadata,
	})
}

func (d Deps) writeProviderError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, compute.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, compute.ErrUnknownKind), errors.Is(err, compute.ErrInUse):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, secret.ErrNoKey):
		// 501, not 400: the request was fine and the deployment is not equipped
		// to serve it. Telling the operator to fix their JSON would be wrong.
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": err.Error()})
	case strings.Contains(err.Error(), "duplicate key"):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "that slug or logical id already exists"})
	default:
		d.Log.Error(operation, "error", err)
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}
