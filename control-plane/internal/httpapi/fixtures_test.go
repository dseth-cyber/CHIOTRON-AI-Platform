package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/ratelimit"
)

const testKey = "ceap_testprefix_testsecret"

type fakeAuthenticator struct {
	identity  auth.Identity
	err       error
	reason    string
	touched   int
	touchErr  error
	presented string
}

func (f *fakeAuthenticator) Authenticate(_ context.Context, presented string) (auth.Identity, string, error) {
	f.presented = presented
	if f.err != nil {
		return auth.Identity{}, f.reason, f.err
	}
	return f.identity, "", nil
}

func (f *fakeAuthenticator) TouchLastUsed(context.Context, string) error {
	f.touched++
	return f.touchErr
}

type fakeLimiter struct {
	allowed bool
	limit   int
	err     error
	calls   int
}

func (f *fakeLimiter) Allow(context.Context, string, int) (ratelimit.Decision, error) {
	f.calls++
	if f.err != nil {
		return ratelimit.Decision{}, f.err
	}
	remaining := 0
	if f.allowed {
		remaining = f.limit - 1
	}
	return ratelimit.Decision{
		Allowed:   f.allowed,
		Limit:     f.limit,
		Remaining: remaining,
		ResetAt:   time.Now().Add(30 * time.Second),
	}, nil
}

type fakeAudit struct {
	events []audit.Event
	usage  []audit.Usage
}

func (f *fakeAudit) Record(_ context.Context, event audit.Event) { f.events = append(f.events, event) }

func (f *fakeAudit) RecordUsage(_ context.Context, usage audit.Usage) {
	f.usage = append(f.usage, usage)
}

func (f *fakeAudit) PendingCounts(context.Context) (int, int, error) {
	return len(f.events), len(f.usage), nil
}

// lastEvent returns the most recent audit event, failing the test if none was
// recorded.
func (f *fakeAudit) lastEvent(t *testing.T) audit.Event {
	t.Helper()
	if len(f.events) == 0 {
		t.Fatal("no audit event was recorded")
	}
	return f.events[len(f.events)-1]
}

func fullyScopedIdentity() auth.Identity {
	return auth.Identity{
		KeyID:              "11111111-1111-1111-1111-111111111111",
		Name:               "test",
		Scopes:             auth.KnownScopes,
		RateLimitPerMinute: 60,
	}
}

func allowingLimiter() *fakeLimiter { return &fakeLimiter{allowed: true, limit: 60} }

// authedGet issues a request carrying a bearer token.
func authedGet(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	return get(t, handler, path, map[string]string{"Authorization": "Bearer " + testKey})
}

func authedPost(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
