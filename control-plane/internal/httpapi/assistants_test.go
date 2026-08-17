package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/auth"
)

func sampleAssistant() assistant.Assistant {
	return assistant.Assistant{
		ID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", Slug: "general", Name: "General assistant",
		Description: "Answers general questions.", Instructions: "You are the enterprise assistant.",
		LogicalModel: "default", Enabled: true,
	}
}

func assistantFixture(catalogue *fakeAssistants, mutate ...func(*Deps)) (http.Handler, *fakeAudit) {
	recorder := &fakeAudit{}
	deps := Deps{
		Config:     testConfig(),
		Log:        quietLogger(),
		Auth:       &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter:    allowingLimiter(),
		Audit:      recorder,
		Assistants: catalogue,
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return NewRouter(deps), recorder
}

// Instructions are assistant policy the operator wrote. The catalogue lists what
// to pick, not the prompt behind it.
func TestAssistantCatalogueOmitsInstructions(t *testing.T) {
	handler, _ := assistantFixture(&fakeAssistants{records: []assistant.Assistant{sampleAssistant()}})

	rec := authedGet(t, handler, "/api/v1/assistants")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); strings.Contains(body, "enterprise assistant") {
		t.Errorf("catalogue leaked assistant instructions: %s", body)
	}

	entry := decode(t, rec)["assistants"].([]any)[0].(map[string]any)
	if entry["slug"] != "general" || entry["logicalModel"] != "default" {
		t.Errorf("assistant entry = %v, want the slug and logical model", entry)
	}
}

// The company predicate is the caller's, never one the caller supplies.
func TestAssistantCatalogueFiltersByCallerCompany(t *testing.T) {
	catalogue := &fakeAssistants{records: []assistant.Assistant{sampleAssistant()}}
	handler, _ := assistantFixture(catalogue, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: auth.KnownScopes, CompanyID: "acme", RateLimitPerMinute: 60,
		}}
	})

	authedGet(t, handler, "/api/v1/assistants")
	if catalogue.company != "acme" {
		t.Errorf("catalogue was queried for company %q, want acme", catalogue.company)
	}
}

func TestAssistantCatalogueRequiresScope(t *testing.T) {
	handler, _ := assistantFixture(&fakeAssistants{}, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: []string{auth.ScopeChatCompletion}, RateLimitPerMinute: 60,
		}}
	})

	if rec := authedGet(t, handler, "/api/v1/assistants"); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 without assistants:read", rec.Code)
	}
}

func TestCreateAssistant(t *testing.T) {
	catalogue := &fakeAssistants{}
	handler, recorder := assistantFixture(catalogue)

	rec := authedPost(t, handler, "/api/v1/admin/assistants",
		`{"slug":"support","name":"Support","logicalModel":"default","instructions":"Be helpful."}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}
	if catalogue.created.Slug != "support" || catalogue.created.LogicalModel != "default" {
		t.Errorf("created params = %+v, want the submitted slug and model", catalogue.created)
	}
	if catalogue.created.CreatedBy != fullyScopedIdentity().KeyID {
		t.Errorf("createdBy = %q, want the calling key", catalogue.created.CreatedBy)
	}
	if event := recorder.lastEvent(t); event.Action != "assistant.created" {
		t.Errorf("audit action = %q, want assistant.created", event.Action)
	}
}

// A key scoped to one company must not be able to plant an assistant in another.
func TestCreateAssistantRejectsForeignCompany(t *testing.T) {
	handler, _ := assistantFixture(&fakeAssistants{}, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: auth.KnownScopes, CompanyID: "acme", RateLimitPerMinute: 60,
		}}
	})

	rec := authedPost(t, handler, "/api/v1/admin/assistants",
		`{"slug":"x","name":"X","logicalModel":"default","companyId":"other"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// A company-scoped key creating an assistant without naming a company gets its
// own, rather than a platform-wide one.
func TestCreateAssistantDefaultsToCallerCompany(t *testing.T) {
	catalogue := &fakeAssistants{}
	handler, _ := assistantFixture(catalogue, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: auth.KnownScopes, CompanyID: "acme", RateLimitPerMinute: 60,
		}}
	})

	authedPost(t, handler, "/api/v1/admin/assistants", `{"slug":"x","name":"X","logicalModel":"default"}`)
	if catalogue.created.CompanyID != "acme" {
		t.Errorf("companyId = %q, want the caller's company", catalogue.created.CompanyID)
	}
}

func TestCreateAssistantRequiresAdminScope(t *testing.T) {
	handler, _ := assistantFixture(&fakeAssistants{}, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: []string{auth.ScopeAssistantsRead}, RateLimitPerMinute: 60,
		}}
	})

	rec := authedPost(t, handler, "/api/v1/admin/assistants", `{"slug":"x","name":"X","logicalModel":"default"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 without admin:assistants", rec.Code)
	}
}
