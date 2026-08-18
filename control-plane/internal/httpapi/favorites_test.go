package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/favorite"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
)

type fakeFavorites struct {
	records []favorite.Favorite
	err     error
	// added and removed record the last call, so a test can prove the handler
	// passed the caller's own identity rather than anything from the body.
	added   []string
	removed []string
	// classifications records the clearance the list was resolved against.
	classifications []string
}

func (f *fakeFavorites) Add(_ context.Context, actorID, companyID, kind, targetID string) error {
	if f.err != nil {
		return f.err
	}
	f.added = []string{actorID, companyID, kind, targetID}
	return nil
}

func (f *fakeFavorites) Remove(_ context.Context, actorID, kind, targetID string) error {
	if f.err != nil {
		return f.err
	}
	f.removed = []string{actorID, kind, targetID}
	return nil
}

func (f *fakeFavorites) List(_ context.Context, _, _ string, classifications []string) ([]favorite.Favorite, error) {
	f.classifications = classifications
	return f.records, f.err
}

func (f *fakeFavorites) IDs(context.Context, string, string) ([]string, error) { return nil, nil }

func favoriteFixture(t *testing.T, mutate ...func(*Deps)) (http.Handler, *fakeFavorites) {
	t.Helper()

	policy, err := knowledge.NewPolicy([]string{"public", "internal", "confidential", "restricted"})
	if err != nil {
		t.Fatalf("NewPolicy() returned error: %v", err)
	}

	store := &fakeFavorites{}
	identity := fullyScopedIdentity()
	identity.MaxClassification = "internal"
	identity.CompanyID = "acme"

	deps := Deps{
		Config:    testConfig(),
		Log:       quietLogger(),
		Auth:      &fakeAuthenticator{identity: identity},
		Limiter:   allowingLimiter(),
		Audit:     &fakeAudit{},
		Favorites: store,
		Knowledge: Knowledge{Policy: policy},
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return NewRouter(deps), store
}

func authedSend(t *testing.T, handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+testKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestFavoriteListResolvesAgainstTheCallersClearance(t *testing.T) {
	handler, store := favoriteFixture(t)
	store.records = []favorite.Favorite{{
		Kind: favorite.KindDocument, TargetID: "d1", Label: "Runbook",
		Detail: "internal", CreatedAt: time.Now(),
	}}

	rec := authedGet(t, handler, "/api/v1/favorites")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /favorites = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Favorites []favorite.Favorite `json:"favorites"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Favorites) != 1 || body.Favorites[0].Label != "Runbook" {
		t.Fatalf("favorites = %+v, want the one stored mark", body.Favorites)
	}

	// A mark must not become a way to read a title above the caller's clearance,
	// so the store is asked for exactly what this key may read.
	if len(store.classifications) == 0 {
		t.Fatal("List was called with no classification predicate")
	}
	for _, level := range store.classifications {
		if level == "confidential" || level == "restricted" {
			t.Fatalf("classifications = %v, which is above an internal clearance", store.classifications)
		}
	}
}

func TestFavoriteAddUsesTheCallersIdentity(t *testing.T) {
	handler, store := favoriteFixture(t)

	rec := authedSend(t, handler, http.MethodPut, "/api/v1/favorites",
		`{"kind":"assistant","targetId":"a1"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PUT /favorites = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	want := []string{"11111111-1111-1111-1111-111111111111", "acme", "assistant", "a1"}
	for index, value := range want {
		if store.added[index] != value {
			t.Fatalf("Add called with %v, want %v", store.added, want)
		}
	}
}

func TestFavoriteRemoveIsScopedToTheCaller(t *testing.T) {
	handler, store := favoriteFixture(t)

	rec := authedSend(t, handler, http.MethodDelete, "/api/v1/favorites",
		`{"kind":"conversation","targetId":"c1"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /favorites = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if store.removed[0] != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("Remove actor = %q, want the calling key", store.removed[0])
	}
}

func TestFavoriteRejectsAnUnknownKind(t *testing.T) {
	// A typo would otherwise be stored as a mark nothing can ever resolve, so the
	// store refuses it and the handler has to report that as the caller's mistake.
	handler, store := favoriteFixture(t)
	store.err = favorite.ErrUnknownKind

	rec := authedSend(t, handler, http.MethodPut, "/api/v1/favorites",
		`{"kind":"invoice","targetId":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT with unknown kind = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestFavoriteRequiresATarget(t *testing.T) {
	handler, _ := favoriteFixture(t)

	rec := authedSend(t, handler, http.MethodPut, "/api/v1/favorites", `{"kind":"assistant"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT with no target = %d, want 400: %s", rec.Code, rec.Body.String())
	}
}

func TestFavoritesNeedTheChatScope(t *testing.T) {
	handler, _ := favoriteFixture(t, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", RateLimitPerMinute: 60, MaxClassification: "internal",
			Scopes: []string{auth.ScopeKnowledgeRead},
		}}
	})

	rec := authedGet(t, handler, "/api/v1/favorites")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("GET /favorites without chat:completions = %d, want 403", rec.Code)
	}
}
