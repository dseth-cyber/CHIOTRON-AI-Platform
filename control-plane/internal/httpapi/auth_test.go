package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
)

func guardedHandler(d Deps) http.Handler {
	if d.Config.ServiceName == "" {
		d.Config = testConfig()
	}
	if d.Log == nil {
		d.Log = quietLogger()
	}
	return d.guard(auth.ScopeModelsRead, func(w http.ResponseWriter, r *http.Request) {
		identity, ok := auth.IdentityFrom(r.Context())
		if !ok {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "identity missing from context"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"keyId": identity.KeyID})
	})
}

func guardFixture(mutate ...func(*Deps)) (http.Handler, *fakeAudit, *fakeLimiter) {
	recorder := &fakeAudit{}
	limiter := allowingLimiter()
	deps := Deps{
		Config:  testConfig(),
		Log:     quietLogger(),
		Auth:    &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter: limiter,
		Audit:   recorder,
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return guardedHandler(deps), recorder, limiter
}

func TestGuardRejectsMissingCredential(t *testing.T) {
	handler, _, limiter := guardFixture()

	rec := get(t, handler, "/guarded", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("401 response is missing WWW-Authenticate")
	}
	// An unauthenticated request must not consume quota.
	if limiter.calls != 0 {
		t.Errorf("limiter was called %d times for an unauthenticated request, want 0", limiter.calls)
	}
}

func TestGuardRejectsMalformedAuthorizationHeader(t *testing.T) {
	handler, _, _ := guardFixture()

	for _, header := range []string{"Basic abc", "Bearer", "Bearer   ", "ceap_x_y"} {
		rec := get(t, handler, "/guarded", map[string]string{"Authorization": header})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Authorization %q = %d, want 401", header, rec.Code)
		}
	}
}

// The client is told the key is invalid and nothing more; the specific reason
// stays server-side so a probe cannot enumerate prefixes.
func TestGuardDoesNotLeakRejectionReason(t *testing.T) {
	handler, _, _ := guardFixture(func(d *Deps) {
		d.Auth = &fakeAuthenticator{err: auth.ErrInvalidKey, reason: "key revoked"}
	})

	rec := authedGet(t, handler, "/guarded")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if body := rec.Body.String(); strings.Contains(body, "revoked") {
		t.Errorf("response leaked the server-side reason: %s", body)
	}
}

func TestGuardRequiresScope(t *testing.T) {
	handler, recorder, _ := guardFixture(func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: []string{auth.ScopeChatCompletion}, RateLimitPerMinute: 60,
		}}
	})

	rec := authedGet(t, handler, "/guarded")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	event := recorder.lastEvent(t)
	if event.Action != "request.denied" || event.Outcome != audit.OutcomeDenied {
		t.Errorf("audit event = %+v, want a denied request.denied event", event)
	}
}

// X-Active-Company is honoured only after the credential is validated and only
// when it matches the credential's entitlement (ARCHITECTURE-v1 section 5).
func TestGuardChecksActiveCompany(t *testing.T) {
	withCompany := func(company string) func(*Deps) {
		return func(d *Deps) {
			d.Auth = &fakeAuthenticator{identity: auth.Identity{
				KeyID: "k", Scopes: auth.KnownScopes, CompanyID: company, RateLimitPerMinute: 60,
			}}
		}
	}

	cases := map[string]struct {
		keyCompany string
		header     string
		want       int
	}{
		"matching company":   {"acme", "acme", http.StatusOK},
		"different company":  {"acme", "other", http.StatusForbidden},
		"key has no company": {"", "acme", http.StatusForbidden},
		"header absent":      {"acme", "", http.StatusOK},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			handler, _, _ := guardFixture(withCompany(tc.keyCompany))
			headers := map[string]string{"Authorization": "Bearer " + testKey}
			if tc.header != "" {
				headers["X-Active-Company"] = tc.header
			}
			if rec := get(t, handler, "/guarded", headers); rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestGuardEnforcesRateLimit(t *testing.T) {
	handler, recorder, _ := guardFixture(func(d *Deps) {
		d.Limiter = &fakeLimiter{allowed: false, limit: 60}
	})

	rec := authedGet(t, handler, "/guarded")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 response is missing Retry-After")
	}
	if rec.Header().Get("X-RateLimit-Limit") != "60" {
		t.Errorf("X-RateLimit-Limit = %q, want 60", rec.Header().Get("X-RateLimit-Limit"))
	}
	if event := recorder.lastEvent(t); event.Outcome != audit.OutcomeDenied {
		t.Errorf("throttled request recorded outcome %q, want denied", event.Outcome)
	}
}

func TestGuardPublishesRateLimitHeadersOnSuccess(t *testing.T) {
	handler, _, _ := guardFixture()

	rec := authedGet(t, handler, "/guarded")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	for _, header := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"} {
		if rec.Header().Get(header) == "" {
			t.Errorf("successful response is missing %s", header)
		}
	}
}

// The limiter protects the compute plane, so an outage sheds load rather than
// silently removing the quota ceiling.
func TestGuardFailsClosedWhenLimiterIsDown(t *testing.T) {
	handler, _, _ := guardFixture(func(d *Deps) {
		d.Limiter = &fakeLimiter{err: errors.New("redis down")}
	})

	if rec := authedGet(t, handler, "/guarded"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

// Failing to record activity is not worth failing a request over.
func TestGuardToleratesActivityTrackingFailure(t *testing.T) {
	handler, _, _ := guardFixture(func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: fullyScopedIdentity(), touchErr: errors.New("write failed")}
	})

	if rec := authedGet(t, handler, "/guarded"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestGuardPassesIdentityDownstream(t *testing.T) {
	handler, _, _ := guardFixture()

	rec := authedGet(t, handler, "/guarded")
	if got := decode(t, rec)["keyId"]; got != fullyScopedIdentity().KeyID {
		t.Errorf("handler saw keyId %v, want the authenticated key", got)
	}
}

// A client must be able to discover its own capabilities without holding any
// particular one of them.
func TestMeReturnsCallerIdentity(t *testing.T) {
	handler := NewRouter(Deps{
		Config:  testConfig(),
		Log:     quietLogger(),
		Auth:    &fakeAuthenticator{identity: auth.Identity{KeyID: "k", Name: "portal", Scopes: []string{auth.ScopeModelsRead}, RateLimitPerMinute: 30}},
		Limiter: allowingLimiter(),
		Audit:   &fakeAudit{},
	})

	rec := authedGet(t, handler, "/api/v1/me")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 for any authenticated caller", rec.Code)
	}
	body := decode(t, rec)
	if body["name"] != "portal" {
		t.Errorf("name = %v, want portal", body["name"])
	}
	scopes := body["scopes"].([]any)
	if len(scopes) != 1 || scopes[0] != auth.ScopeModelsRead {
		t.Errorf("scopes = %v, want the granted scopes", scopes)
	}
	if body["rateLimitPerMinute"] != float64(30) {
		t.Errorf("rateLimitPerMinute = %v, want 30", body["rateLimitPerMinute"])
	}
}

func TestMeStillRequiresACredential(t *testing.T) {
	router := NewRouter(Deps{
		Config: testConfig(), Log: quietLogger(),
		Auth: &fakeAuthenticator{identity: fullyScopedIdentity()}, Limiter: allowingLimiter(), Audit: &fakeAudit{},
	})
	if rec := get(t, router, "/api/v1/me", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 without a key", rec.Code)
	}
}

// Operational endpoints stay open: probes and scrapes have no credential.
func TestOperationalEndpointsStayPublic(t *testing.T) {
	handler := NewRouter(Deps{
		Config:  testConfig(),
		Log:     quietLogger(),
		Metrics: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("# metrics")) }),
		Auth:    &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter: allowingLimiter(),
		Audit:   &fakeAudit{},
	})

	for _, path := range []string{"/healthz", "/readyz", "/metrics", "/api/v1/platform"} {
		if rec := get(t, handler, path, nil); rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200 without a credential", path, rec.Code)
		}
	}
}
