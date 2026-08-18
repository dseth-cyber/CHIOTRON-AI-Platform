package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/conversation"
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

type fakeAssistants struct {
	records []assistant.Assistant
	err     error
	created assistant.CreateParams
	// company records which company predicate the catalogue was asked for.
	company string
}

func (f *fakeAssistants) List(_ context.Context, companyID string) ([]assistant.Assistant, error) {
	f.company = companyID
	return f.records, f.err
}

func (f *fakeAssistants) Resolve(_ context.Context, reference, companyID string) (assistant.Assistant, error) {
	f.company = companyID
	if f.err != nil {
		return assistant.Assistant{}, f.err
	}
	for _, record := range f.records {
		if record.Slug == reference || record.ID == reference {
			return record, nil
		}
	}
	return assistant.Assistant{}, fmt.Errorf("%w: %q", assistant.ErrNotFound, reference)
}

func (f *fakeAssistants) Create(_ context.Context, params assistant.CreateParams) (assistant.Assistant, error) {
	f.created = params
	if f.err != nil {
		return assistant.Assistant{}, f.err
	}
	return assistant.Assistant{ID: "aaaa", Slug: params.Slug, Name: params.Name, LogicalModel: params.LogicalModel, CompanyID: params.CompanyID}, nil
}

type fakeConversations struct {
	record    conversation.Conversation
	history   []conversation.Message
	turns     []conversation.Turn
	created   int
	deleted   string
	getErr    error
	appendErr error
	persist   bool
	restored  string
	// trash is what ListDeleted returns, and listedTrash records that it was the
	// listing the handler chose.
	trash       []conversation.Conversation
	listedTrash bool
	// actors records which owner each read was scoped to.
	actors []string
}

func (f *fakeConversations) Create(_ context.Context, _ conversation.Owner, assistantID string) (conversation.Conversation, error) {
	f.created++
	f.record.AssistantID = assistantID
	if f.record.ID == "" {
		f.record.ID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	}
	return f.record, nil
}

func (f *fakeConversations) List(_ context.Context, actorID string, _ int) ([]conversation.Conversation, error) {
	f.actors = append(f.actors, actorID)
	if f.record.ID == "" {
		return nil, nil
	}
	return []conversation.Conversation{f.record}, nil
}

func (f *fakeConversations) ListDeleted(_ context.Context, actorID string, _ int) ([]conversation.Conversation, error) {
	f.actors = append(f.actors, actorID)
	f.listedTrash = true
	return f.trash, nil
}

func (f *fakeConversations) Restore(_ context.Context, id, actorID string) error {
	f.actors = append(f.actors, actorID)
	if f.getErr != nil {
		return f.getErr
	}
	f.restored = id
	return nil
}

func (f *fakeConversations) Get(_ context.Context, _, actorID string) (conversation.Conversation, error) {
	f.actors = append(f.actors, actorID)
	if f.getErr != nil {
		return conversation.Conversation{}, f.getErr
	}
	return f.record, nil
}

func (f *fakeConversations) Messages(_ context.Context, _, actorID string) ([]conversation.Message, error) {
	f.actors = append(f.actors, actorID)
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.history, nil
}

func (f *fakeConversations) AppendTurn(_ context.Context, _, _ string, turn conversation.Turn) error {
	if f.appendErr != nil {
		return f.appendErr
	}
	f.turns = append(f.turns, turn)
	return nil
}

func (f *fakeConversations) Delete(_ context.Context, id, actorID string) error {
	f.actors = append(f.actors, actorID)
	if f.getErr != nil {
		return f.getErr
	}
	f.deleted = id
	return nil
}

func (f *fakeConversations) PersistsContent() bool { return f.persist }

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
