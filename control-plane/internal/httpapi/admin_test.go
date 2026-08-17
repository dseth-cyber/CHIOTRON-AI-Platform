package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/auth"
)

type fakeKeyAdmin struct {
	created auth.CreateParams
	record  auth.Record
	secret  string
	list    []auth.Record
	revoked string
	err     error
}

func (f *fakeKeyAdmin) Create(_ context.Context, params auth.CreateParams) (auth.Record, string, error) {
	f.created = params
	if f.err != nil {
		return auth.Record{}, "", f.err
	}
	return f.record, f.secret, nil
}

func (f *fakeKeyAdmin) List(context.Context) ([]auth.Record, error) {
	return f.list, f.err
}

func (f *fakeKeyAdmin) Revoke(_ context.Context, id string) (auth.Record, error) {
	f.revoked = id
	if f.err != nil {
		return auth.Record{}, f.err
	}
	return f.record, nil
}

func adminFixture(keys *fakeKeyAdmin, mutate ...func(*Deps)) (http.Handler, *fakeAudit) {
	recorder := &fakeAudit{}
	deps := Deps{
		Config:  testConfig(),
		Log:     quietLogger(),
		Auth:    &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter: allowingLimiter(),
		Audit:   recorder,
		Keys:    keys,
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return NewRouter(deps), recorder
}

func sampleRecord() auth.Record {
	return auth.Record{
		ID: "22222222-2222-2222-2222-222222222222", Name: "portal",
		Prefix: "abc123", Scopes: []string{auth.ScopeModelsRead}, RateLimitPerMinute: 60,
	}
}

// The raw value is shown once only, so the create response is the single
// opportunity to hand it over (ARCHITECTURE-v1 section 5).
func TestCreateKeyReturnsSecretOnce(t *testing.T) {
	keys := &fakeKeyAdmin{record: sampleRecord(), secret: "ceap_abc123_supersecret"}
	handler, recorder := adminFixture(keys)

	rec := authedPost(t, handler, "/api/v1/admin/api-keys", `{"name":"portal","scopes":["models:read"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (%s)", rec.Code, rec.Body.String())
	}

	body := decode(t, rec)
	if body["secret"] != "ceap_abc123_supersecret" {
		t.Errorf("secret = %v, want the generated value", body["secret"])
	}
	if body["notice"] == nil {
		t.Error("response does not warn that the secret cannot be retrieved again")
	}

	// The listing view must never contain a secret.
	key := body["apiKey"].(map[string]any)
	if _, leaked := key["secret"]; leaked {
		t.Error("api key record carries a secret field")
	}
	if _, leaked := key["secretHash"]; leaked {
		t.Error("api key record carries the stored hash")
	}

	if event := recorder.lastEvent(t); event.Action != "api_key.created" {
		t.Errorf("audit action = %q, want api_key.created", event.Action)
	}
}

func TestCreateKeyAppliesDefaultRateLimit(t *testing.T) {
	keys := &fakeKeyAdmin{record: sampleRecord(), secret: "s"}
	handler, _ := adminFixture(keys, func(d *Deps) {
		cfg := testConfig()
		cfg.DefaultRateLimitPerMinute = 42
		d.Config = cfg
	})

	authedPost(t, handler, "/api/v1/admin/api-keys", `{"name":"portal","scopes":["models:read"]}`)
	if keys.created.RateLimitPerMinute != 42 {
		t.Errorf("rate limit = %d, want the configured default of 42", keys.created.RateLimitPerMinute)
	}
}

func TestCreateKeyRecordsCallerAsCreator(t *testing.T) {
	keys := &fakeKeyAdmin{record: sampleRecord(), secret: "s"}
	handler, _ := adminFixture(keys)

	authedPost(t, handler, "/api/v1/admin/api-keys", `{"name":"portal","scopes":["models:read"]}`)
	if keys.created.CreatedBy != fullyScopedIdentity().KeyID {
		t.Errorf("createdBy = %q, want the calling key", keys.created.CreatedBy)
	}
}

func TestCreateKeyRejectsBadInput(t *testing.T) {
	handler, _ := adminFixture(&fakeKeyAdmin{err: errors.New("unknown scope")})

	cases := map[string]string{
		"unknown field": `{"name":"x","scopes":["models:read"],"admin":true}`,
		"bad expiry":    `{"name":"x","scopes":["models:read"],"expiresAt":"tomorrow"}`,
		"store refused": `{"name":"x","scopes":["nope"]}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := authedPost(t, handler, "/api/v1/admin/api-keys", body); rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestListKeysNeverExposesSecrets(t *testing.T) {
	handler, _ := adminFixture(&fakeKeyAdmin{list: []auth.Record{sampleRecord()}})

	rec := authedGet(t, handler, "/api/v1/admin/api-keys")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "secret") {
		t.Errorf("listing mentions a secret: %s", body)
	}
}

func TestRevokeKey(t *testing.T) {
	keys := &fakeKeyAdmin{record: sampleRecord()}
	handler, recorder := adminFixture(keys)

	rec := authedPost(t, handler, "/api/v1/admin/api-keys/"+sampleRecord().ID+"/revoke", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if keys.revoked != sampleRecord().ID {
		t.Errorf("revoked %q, want the id from the path", keys.revoked)
	}
	if event := recorder.lastEvent(t); event.Action != "api_key.revoked" {
		t.Errorf("audit action = %q, want api_key.revoked", event.Action)
	}
}

func TestRevokeUnknownKeyIs404(t *testing.T) {
	handler, _ := adminFixture(&fakeKeyAdmin{err: auth.ErrKeyNotFound})

	rec := authedPost(t, handler, "/api/v1/admin/api-keys/does-not-exist/revoke", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// Minting keys is the most dangerous capability in the platform, so it must be
// gated by its own scope rather than by being an authenticated caller.
func TestAdminRoutesRequireAdminScope(t *testing.T) {
	handler, _ := adminFixture(&fakeKeyAdmin{record: sampleRecord()}, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: []string{auth.ScopeModelsRead, auth.ScopeChatCompletion}, RateLimitPerMinute: 60,
		}}
	})

	if rec := authedGet(t, handler, "/api/v1/admin/api-keys"); rec.Code != http.StatusForbidden {
		t.Errorf("list without admin:keys = %d, want 403", rec.Code)
	}
	rec := authedPost(t, handler, "/api/v1/admin/api-keys", `{"name":"x","scopes":["models:read"]}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("create without admin:keys = %d, want 403", rec.Code)
	}
}

func TestOutboxReportsBacklog(t *testing.T) {
	handler, _ := adminFixture(&fakeKeyAdmin{})

	rec := authedGet(t, handler, "/api/v1/admin/outbox")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decode(t, rec)
	if body["pending"] == nil || body["topics"] == nil {
		t.Errorf("response = %v, want pending counts and their target topics", body)
	}
}
