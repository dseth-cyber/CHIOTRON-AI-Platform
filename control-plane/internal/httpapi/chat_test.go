package httpapi

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/conversation"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

var errStorage = errors.New("storage unavailable")

// recordingProvider remembers what the gateway actually asked the model.
type recordingProvider struct {
	fakeProvider
	sent provider.ChatRequest
}

func (r *recordingProvider) Chat(ctx context.Context, req provider.ChatRequest) (provider.ChatResponse, error) {
	r.sent = req
	return r.fakeProvider.Chat(ctx, req)
}

type statefulFixture struct {
	handler       http.Handler
	compute       *recordingProvider
	assistants    *fakeAssistants
	conversations *fakeConversations
	audit         *fakeAudit
}

func newStatefulFixture(t *testing.T, mutate ...func(*Deps)) statefulFixture {
	t.Helper()
	compute := &recordingProvider{fakeProvider: fakeProvider{
		name: "ollama", models: []provider.Model{{ID: "qwen2.5:0.5b"}},
	}}
	routes, err := provider.ParseRoutes("default=ollama/qwen2.5:0.5b")
	if err != nil {
		t.Fatalf("ParseRoutes() returned error: %v", err)
	}
	registry, err := provider.NewRegistry(routes, "default", compute)
	if err != nil {
		t.Fatalf("NewRegistry() returned error: %v", err)
	}

	catalogue := &fakeAssistants{records: []assistant.Assistant{sampleAssistant()}}
	store := &fakeConversations{record: sampleConversation(), persist: true}
	recorder := &fakeAudit{}

	deps := Deps{
		Config:        testConfig(),
		Log:           quietLogger(),
		Compute:       registry,
		Auth:          &fakeAuthenticator{identity: fullyScopedIdentity()},
		Limiter:       allowingLimiter(),
		Audit:         recorder,
		Assistants:    catalogue,
		Conversations: store,
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return statefulFixture{
		handler: NewRouter(deps), compute: compute,
		assistants: catalogue, conversations: store, audit: recorder,
	}
}

// Selecting an assistant is what hides provider details from the user: the
// assistant supplies the instructions and the logical model.
func TestStatefulChatPrependsAssistantInstructions(t *testing.T) {
	fixture := newStatefulFixture(t)

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"assistant":"general","message":"How do I rotate a key?"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	sent := fixture.compute.sent.Messages
	if len(sent) < 2 {
		t.Fatalf("sent %d messages, want instructions plus the question", len(sent))
	}
	if sent[0].Role != "system" || sent[0].Content != sampleAssistant().Instructions {
		t.Errorf("first message = %+v, want the assistant instructions as a system message", sent[0])
	}
	if last := sent[len(sent)-1]; last.Role != "user" || last.Content != "How do I rotate a key?" {
		t.Errorf("last message = %+v, want the new question", last)
	}

	body := decode(t, rec)
	if body["conversationId"] != sampleConversation().ID {
		t.Errorf("conversationId = %v, want the stored conversation", body["conversationId"])
	}
	if body["assistant"] != "general" {
		t.Errorf("assistant = %v, want general", body["assistant"])
	}
}

func TestStatefulChatCreatesConversationAndRecordsTurn(t *testing.T) {
	fixture := newStatefulFixture(t)

	authedPost(t, fixture.handler, "/api/v1/chat/completions", `{"assistant":"general","message":"hi"}`)

	if fixture.conversations.created != 1 {
		t.Errorf("created %d conversations, want 1", fixture.conversations.created)
	}
	if len(fixture.conversations.turns) != 1 {
		t.Fatalf("recorded %d turns, want 1", len(fixture.conversations.turns))
	}
	turn := fixture.conversations.turns[0]
	if turn.Question != "hi" || turn.Answer != "hello" {
		t.Errorf("turn = %+v, want the question and the model's answer", turn)
	}
	if turn.PromptTokens != 11 || turn.CompletionTokens != 4 {
		t.Errorf("turn tokens = %+v, want the provider's counts", turn)
	}
}

// Resuming a conversation replays its transcript, so the model sees the context
// without the client having to send it back.
func TestStatefulChatReplaysHistory(t *testing.T) {
	fixture := newStatefulFixture(t, func(d *Deps) {
		d.Conversations = &fakeConversations{
			record:  sampleConversation(),
			persist: true,
			history: []conversation.Message{
				{Role: "user", Content: "first question"},
				{Role: "assistant", Content: "first answer"},
			},
		}
	})

	authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"conversationId":"`+sampleConversation().ID+`","message":"follow up"}`)

	sent := fixture.compute.sent.Messages
	if len(sent) != 4 {
		t.Fatalf("sent %d messages, want instructions, two history turns and the question: %+v", len(sent), sent)
	}
	if sent[1].Content != "first question" || sent[2].Content != "first answer" {
		t.Errorf("history was not replayed in order: %+v", sent)
	}
	if fixture.conversations.created != 0 {
		t.Error("resuming a conversation created a new one")
	}
}

// A redacted turn has no text to replay; sending it empty would waste context.
func TestStatefulChatSkipsRedactedHistory(t *testing.T) {
	fixture := newStatefulFixture(t, func(d *Deps) {
		d.Conversations = &fakeConversations{
			record: sampleConversation(),
			history: []conversation.Message{
				{Role: "user", Content: "", Redacted: true},
				{Role: "assistant", Content: "", Redacted: true},
			},
		}
	})

	authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"conversationId":"`+sampleConversation().ID+`","message":"follow up"}`)

	for _, message := range fixture.compute.sent.Messages {
		if message.Content == "" {
			t.Errorf("an empty message reached the model: %+v", fixture.compute.sent.Messages)
		}
	}
}

// The assistant's tuning is the default, and an explicit value still wins.
func TestStatefulChatUsesAssistantModelAndTuning(t *testing.T) {
	temperature := 0.9
	maxTokens := 32
	tuned := sampleAssistant()
	tuned.LogicalModel = "default"
	tuned.Temperature = &temperature
	tuned.MaxTokens = &maxTokens

	fixture := newStatefulFixture(t, func(d *Deps) {
		d.Assistants = &fakeAssistants{records: []assistant.Assistant{tuned}}
	})

	authedPost(t, fixture.handler, "/api/v1/chat/completions", `{"assistant":"general","message":"hi"}`)
	if fixture.compute.sent.Temperature == nil || *fixture.compute.sent.Temperature != 0.9 {
		t.Errorf("temperature = %v, want the assistant's 0.9", fixture.compute.sent.Temperature)
	}
	if fixture.compute.sent.MaxTokens == nil || *fixture.compute.sent.MaxTokens != 32 {
		t.Errorf("maxTokens = %v, want the assistant's 32", fixture.compute.sent.MaxTokens)
	}

	authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"assistant":"general","message":"hi","temperature":0.1}`)
	if fixture.compute.sent.Temperature == nil || *fixture.compute.sent.Temperature != 0.1 {
		t.Errorf("temperature = %v, want the request's 0.1 to win", fixture.compute.sent.Temperature)
	}
}

func TestStatefulChatRejectsAmbiguousRequests(t *testing.T) {
	fixture := newStatefulFixture(t)

	cases := map[string]string{
		"both forms":        `{"assistant":"general","message":"hi","messages":[{"role":"user","content":"hi"}]}`,
		"empty message":     `{"assistant":"general","message":""}`,
		"no assistant":      `{"message":"hi"}`,
		"unknown assistant": `{"assistant":"nope","message":"hi"}`,
	}
	wants := map[string]int{
		"both forms":        http.StatusBadRequest,
		"empty message":     http.StatusBadRequest,
		"no assistant":      http.StatusBadRequest,
		"unknown assistant": http.StatusNotFound,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := authedPost(t, fixture.handler, "/api/v1/chat/completions", body); rec.Code != wants[name] {
				t.Errorf("status = %d, want %d (%s)", rec.Code, wants[name], rec.Body.String())
			}
		})
	}
}

// A conversation is bound to the assistant it started with, so a later request
// cannot quietly switch its behaviour.
func TestStatefulChatRejectsAssistantSwitch(t *testing.T) {
	other := sampleAssistant()
	other.Slug = "support"
	other.ID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	fixture := newStatefulFixture(t, func(d *Deps) {
		d.Assistants = &fakeAssistants{records: []assistant.Assistant{sampleAssistant(), other}}
	})

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"conversationId":"`+sampleConversation().ID+`","assistant":"support","message":"hi"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
	}
}

func TestStatefulChatReportsMissingConversation(t *testing.T) {
	fixture := newStatefulFixture(t, func(d *Deps) {
		d.Conversations = &fakeConversations{getErr: conversation.ErrNotFound}
	})

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"conversationId":"cccccccc-cccc-cccc-cccc-cccccccccccc","message":"hi"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

// The answer has already been produced, so failing to file it must not fail the
// request.
func TestStatefulChatSurvivesStorageFailure(t *testing.T) {
	fixture := newStatefulFixture(t, func(d *Deps) {
		d.Conversations = &fakeConversations{record: sampleConversation(), appendErr: errStorage}
	})

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions", `{"assistant":"general","message":"hi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 despite the storage failure (%s)", rec.Code, rec.Body.String())
	}
	if len(fixture.audit.usage) != 1 {
		t.Errorf("usage was not recorded: %+v", fixture.audit.usage)
	}
}

// The stateless form keeps working for callers that hold their own transcript.
func TestStatelessChatNeedsNoConversation(t *testing.T) {
	fixture := newStatefulFixture(t)

	rec := authedPost(t, fixture.handler, "/api/v1/chat/completions",
		`{"messages":[{"role":"user","content":"hi"}]}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if body := decode(t, rec); body["conversationId"] != nil {
		t.Errorf("stateless call reported conversationId %v, want none", body["conversationId"])
	}
	if fixture.conversations.created != 0 || len(fixture.conversations.turns) != 0 {
		t.Error("a stateless call touched conversation storage")
	}
}
