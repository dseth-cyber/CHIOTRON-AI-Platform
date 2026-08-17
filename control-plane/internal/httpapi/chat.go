package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/conversation"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

// maxChatBody bounds an inbound prompt. Per-key rate limiting caps how often a
// caller may send one; this caps how large each one may be.
const maxChatBody = 1 << 20

// requestError is a failure the caller caused, carrying the status to report.
type requestError struct {
	status  int
	message string
}

func (e requestError) Error() string { return e.message }

func badRequest(format string, args ...any) requestError {
	return requestError{status: http.StatusBadRequest, message: fmt.Sprintf(format, args...)}
}

type chatRequestBody struct {
	// Stateless form: the caller supplies the whole conversation each time.
	Messages []provider.Message `json:"messages,omitempty"`

	// Stateful form: the platform holds the transcript and the assistant
	// supplies the instructions and model.
	Assistant      string `json:"assistant,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	Message        string `json:"message,omitempty"`

	Model       string   `json:"model,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"maxTokens,omitempty"`
	Stream      bool     `json:"stream,omitempty"`
}

// chatPlan is a validated request: what to send, which logical model to send it
// to, and where to record the result.
type chatPlan struct {
	request      provider.ChatRequest
	logicalModel string
	question     string
	assistant    *assistant.Assistant
	conversation *conversation.Conversation
}

// stateful reports whether the result belongs in a stored transcript.
func (p chatPlan) stateful() bool { return p.conversation != nil }

func registerChat(mux *http.ServeMux, d Deps) {
	if d.Compute == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("POST /api/v1/chat/completions", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body chatRequestBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChatBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		plan, err := d.planChat(r.Context(), caller, body)
		if err != nil {
			d.writePlanError(w, err)
			return
		}

		ctx, cancel := contextWithTimeout(r, d.Config.ComputeTimeout)
		defer cancel()

		if body.Stream {
			d.streamChat(ctx, w, r, caller, plan)
			return
		}

		started := time.Now()
		response, route, err := d.Compute.Chat(ctx, plan.logicalModel, plan.request)
		if err != nil {
			// A failed call still consumed capacity and still belongs in the
			// usage record; only its token counts are zero.
			d.Audit.RecordUsage(r.Context(), audit.Usage{
				ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
				LogicalModel: orDefault(route.Logical, plan.logicalModel), Provider: route.Provider, Model: route.Model,
				LatencyMs: time.Since(started).Milliseconds(), Outcome: audit.OutcomeFailure,
			})
			d.writeChatError(w, err, route)
			return
		}

		d.recordCompletion(r.Context(), caller, plan, route, response)
		writeJSON(w, http.StatusOK, d.completionBody(plan, route, response))
	}))
}

// planChat validates the request and assembles the messages to send.
func (d Deps) planChat(ctx context.Context, caller auth.Identity, body chatRequestBody) (chatPlan, error) {
	stateful := body.Assistant != "" || body.ConversationID != "" || body.Message != ""

	switch {
	case stateful && len(body.Messages) > 0:
		return chatPlan{}, badRequest("send either messages for a stateless call or message for a stored conversation, not both")
	case !stateful && len(body.Messages) == 0:
		return chatPlan{}, badRequest("messages must not be empty")
	case !stateful:
		return chatPlan{
			request: provider.ChatRequest{
				Messages: body.Messages, Temperature: body.Temperature, MaxTokens: body.MaxTokens,
			},
			logicalModel: body.Model,
		}, nil
	case body.Message == "":
		return chatPlan{}, badRequest("message must not be empty")
	case d.Assistants == nil || d.Conversations == nil:
		return chatPlan{}, requestError{status: http.StatusNotImplemented, message: "stored conversations are not available"}
	}

	// An existing conversation dictates its assistant; a new one names it.
	reference := body.Assistant
	var existing *conversation.Conversation
	if body.ConversationID != "" {
		record, err := d.Conversations.Get(ctx, body.ConversationID, caller.KeyID)
		if err != nil {
			return chatPlan{}, err
		}
		existing = &record
		if reference != "" && reference != record.AssistantSlug && reference != record.AssistantID {
			return chatPlan{}, badRequest("conversation %s is bound to assistant %q", record.ID, record.AssistantSlug)
		}
		reference = record.AssistantID
	}
	if reference == "" {
		return chatPlan{}, badRequest("assistant is required to start a conversation")
	}

	selected, err := d.Assistants.Resolve(ctx, reference, caller.CompanyID)
	if err != nil {
		return chatPlan{}, err
	}

	if existing == nil {
		record, err := d.Conversations.Create(ctx, conversation.Owner{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
		}, selected.ID)
		if err != nil {
			return chatPlan{}, err
		}
		existing = &record
	}

	messages, err := d.assembleMessages(ctx, caller, *existing, selected, body.Message)
	if err != nil {
		return chatPlan{}, err
	}

	return chatPlan{
		request: provider.ChatRequest{
			Messages:    messages,
			Temperature: firstFloat(body.Temperature, selected.Temperature),
			MaxTokens:   firstInt(body.MaxTokens, selected.MaxTokens),
		},
		logicalModel: orDefault(body.Model, selected.LogicalModel),
		question:     body.Message,
		assistant:    &selected,
		conversation: existing,
	}, nil
}

// assembleMessages builds the model input: assistant instructions, the recent
// transcript, then the new question.
func (d Deps) assembleMessages(ctx context.Context, caller auth.Identity,
	record conversation.Conversation, selected assistant.Assistant, question string) ([]provider.Message, error) {

	history, err := d.Conversations.Messages(ctx, record.ID, caller.KeyID)
	if err != nil {
		return nil, err
	}

	// Only the tail is sent: a transcript grows without limit, a context window
	// does not. A non-positive limit means no cap, so a Config assembled without
	// config.Load cannot silently drop the whole transcript.
	if limit := d.Config.HistoryTurnLimit * 2; limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}

	messages := make([]provider.Message, 0, len(history)+2)
	if selected.Instructions != "" {
		messages = append(messages, provider.Message{Role: "system", Content: selected.Instructions})
	}
	for _, message := range history {
		// A redacted turn has no text to replay. Sending it as an empty message
		// would waste context and confuse the model.
		if message.Redacted || message.Content == "" {
			continue
		}
		messages = append(messages, provider.Message{Role: message.Role, Content: message.Content})
	}
	return append(messages, provider.Message{Role: "user", Content: question}), nil
}

// recordCompletion writes usage metadata and, for a stored conversation, the
// turn itself.
func (d Deps) recordCompletion(ctx context.Context, caller auth.Identity, plan chatPlan,
	route provider.Route, response provider.ChatResponse) {

	d.Audit.RecordUsage(ctx, audit.Usage{
		ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
		LogicalModel: route.Logical, Provider: route.Provider, Model: response.Model,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
		LatencyMs:        response.LatencyMs,
		Outcome:          audit.OutcomeSuccess,
	})

	if !plan.stateful() {
		return
	}
	// The answer has already been delivered, so a storage failure is logged
	// rather than turned into an error the caller cannot act on.
	if err := d.Conversations.AppendTurn(ctx, plan.conversation.ID, caller.KeyID, conversation.Turn{
		Question:         plan.question,
		Answer:           response.Content,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		Model:            response.Model,
	}); err != nil {
		d.Log.Error("record conversation turn", "conversationId", plan.conversation.ID, "error", err)
	}
}

func (d Deps) completionBody(plan chatPlan, route provider.Route, response provider.ChatResponse) map[string]any {
	body := map[string]any{
		"logicalModel": route.Logical,
		"provider":     route.Provider,
		"model":        response.Model,
		"content":      response.Content,
		"finishReason": response.FinishReason,
		"usage":        response.Usage,
		"latencyMs":    response.LatencyMs,
	}
	if plan.stateful() {
		body["conversationId"] = plan.conversation.ID
		body["assistant"] = plan.assistant.Slug
	}
	return body
}

func (d Deps) writePlanError(w http.ResponseWriter, err error) {
	var caused requestError
	switch {
	case errors.As(err, &caused):
		writeJSON(w, caused.status, map[string]string{"error": caused.message})
	case errors.Is(err, conversation.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
	case errors.Is(err, assistant.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		d.Log.Error("plan chat request", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}

func firstFloat(preferred, fallback *float64) *float64 {
	if preferred != nil {
		return preferred
	}
	return fallback
}

func firstInt(preferred, fallback *int) *int {
	if preferred != nil {
		return preferred
	}
	return fallback
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
