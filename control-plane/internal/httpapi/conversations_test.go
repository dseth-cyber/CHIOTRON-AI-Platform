package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/conversation"
)

func sampleConversation() conversation.Conversation {
	return conversation.Conversation{
		ID: "cccccccc-cccc-cccc-cccc-cccccccccccc", Title: "How do I rotate a key?",
		AssistantID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", AssistantSlug: "general",
		AssistantName: "General assistant", MessageCount: 2, TotalTokens: 41,
	}
}

func conversationFixture(store *fakeConversations, mutate ...func(*Deps)) (http.Handler, *fakeAudit) {
	recorder := &fakeAudit{}
	deps := Deps{
		Config:        testConfig(),
		Log:           quietLogger(),
		Auth:          &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter:       allowingLimiter(),
		Audit:         recorder,
		Conversations: store,
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return NewRouter(deps), recorder
}

func authedDelete(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("Authorization", "Bearer "+testKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestListConversations(t *testing.T) {
	store := &fakeConversations{record: sampleConversation(), persist: true}
	handler, _ := conversationFixture(store)

	rec := authedGet(t, handler, "/api/v1/conversations")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if len(body["conversations"].([]any)) != 1 {
		t.Errorf("conversations = %v, want one entry", body["conversations"])
	}
	// Lets a client tell an empty transcript from a redacted one.
	if body["promptsPersisted"] != true {
		t.Errorf("promptsPersisted = %v, want true", body["promptsPersisted"])
	}
}

// A transcript belongs to the credential that created it, so every read is
// scoped to the caller rather than filtered afterwards.
func TestConversationReadsAreScopedToTheCaller(t *testing.T) {
	store := &fakeConversations{record: sampleConversation()}
	handler, _ := conversationFixture(store)

	authedGet(t, handler, "/api/v1/conversations")
	authedGet(t, handler, "/api/v1/conversations/"+sampleConversation().ID)

	if len(store.actors) == 0 {
		t.Fatal("the store was never asked for a specific owner")
	}
	for _, actor := range store.actors {
		if actor != fullyScopedIdentity().KeyID {
			t.Errorf("store was queried for actor %q, want the calling key", actor)
		}
	}
}

func TestGetConversationReturnsMessages(t *testing.T) {
	store := &fakeConversations{
		record:  sampleConversation(),
		history: []conversation.Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hello"}},
		persist: true,
	}
	handler, _ := conversationFixture(store)

	rec := authedGet(t, handler, "/api/v1/conversations/"+sampleConversation().ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if len(body["messages"].([]any)) != 2 {
		t.Errorf("messages = %v, want two", body["messages"])
	}
	if body["conversation"].(map[string]any)["assistantSlug"] != "general" {
		t.Errorf("conversation = %v, want the assistant it is bound to", body["conversation"])
	}
}

// Somebody else's id must read as missing, not forbidden: a 403 would confirm
// the id exists.
func TestUnknownConversationIs404(t *testing.T) {
	store := &fakeConversations{getErr: conversation.ErrNotFound}
	handler, _ := conversationFixture(store)

	if rec := authedGet(t, handler, "/api/v1/conversations/whatever"); rec.Code != http.StatusNotFound {
		t.Errorf("GET status = %d, want 404", rec.Code)
	}
	if rec := authedDelete(t, handler, "/api/v1/conversations/whatever"); rec.Code != http.StatusNotFound {
		t.Errorf("DELETE status = %d, want 404", rec.Code)
	}
}

func TestDeleteConversation(t *testing.T) {
	store := &fakeConversations{record: sampleConversation()}
	handler, recorder := conversationFixture(store)

	rec := authedDelete(t, handler, "/api/v1/conversations/"+sampleConversation().ID)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}
	if store.deleted != sampleConversation().ID {
		t.Errorf("deleted %q, want the id from the path", store.deleted)
	}
	if event := recorder.lastEvent(t); event.Action != "conversation.deleted" {
		t.Errorf("audit action = %q, want conversation.deleted", event.Action)
	}
}

// Reading your own transcript is part of using chat, not a separate capability.
func TestConversationRoutesRequireChatScope(t *testing.T) {
	handler, _ := conversationFixture(&fakeConversations{record: sampleConversation()}, func(d *Deps) {
		d.Auth = &fakeAuthenticator{identity: auth.Identity{
			KeyID: "k", Scopes: []string{auth.ScopeModelsRead}, RateLimitPerMinute: 60,
		}}
	})

	if rec := authedGet(t, handler, "/api/v1/conversations"); rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 without chat:completions", rec.Code)
	}
}

func TestListConversationsRejectsBadLimit(t *testing.T) {
	handler, _ := conversationFixture(&fakeConversations{})

	if rec := authedGet(t, handler, "/api/v1/conversations?limit=0"); rec.Code != http.StatusBadRequest {
		t.Errorf("limit=0 status = %d, want 400", rec.Code)
	}
	if rec := authedGet(t, handler, "/api/v1/conversations?limit=many"); rec.Code != http.StatusBadRequest {
		t.Errorf("limit=many status = %d, want 400", rec.Code)
	}
}
