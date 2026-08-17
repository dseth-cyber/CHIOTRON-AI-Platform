package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/conversation"
)

// ConversationStore holds chat transcripts. Every method is scoped to one
// caller, so an id belonging to somebody else reads as missing.
type ConversationStore interface {
	Create(ctx context.Context, owner conversation.Owner, assistantID string) (conversation.Conversation, error)
	List(ctx context.Context, actorID string, limit int) ([]conversation.Conversation, error)
	Get(ctx context.Context, id, actorID string) (conversation.Conversation, error)
	Messages(ctx context.Context, id, actorID string) ([]conversation.Message, error)
	AppendTurn(ctx context.Context, id, actorID string, turn conversation.Turn) error
	Delete(ctx context.Context, id, actorID string) error
	PersistsContent() bool
}

// registerConversations adds history routes.
//
// They are gated by chat:completions rather than a scope of their own: reading
// your own transcript is part of using chat, and it is never visible to another
// credential.
func registerConversations(mux *http.ServeMux, d Deps) {
	if d.Conversations == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("GET /api/v1/conversations", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be a positive integer"})
				return
			}
			limit = parsed
		}

		records, err := d.Conversations.List(r.Context(), caller.KeyID, limit)
		if err != nil {
			d.Log.Error("list conversations", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if records == nil {
			records = []conversation.Conversation{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"conversations": records,
			// Lets a client tell an empty transcript from a redacted one.
			"promptsPersisted": d.Conversations.PersistsContent(),
		})
	}))

	mux.HandleFunc("GET /api/v1/conversations/{id}", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		id := r.PathValue("id")

		record, err := d.Conversations.Get(r.Context(), id, caller.KeyID)
		if err != nil {
			d.writeConversationError(w, err, "read conversation")
			return
		}
		messages, err := d.Conversations.Messages(r.Context(), id, caller.KeyID)
		if err != nil {
			d.writeConversationError(w, err, "read messages")
			return
		}
		if messages == nil {
			messages = []conversation.Message{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"conversation":     record,
			"messages":         messages,
			"promptsPersisted": d.Conversations.PersistsContent(),
		})
	}))

	mux.HandleFunc("DELETE /api/v1/conversations/{id}", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		id := r.PathValue("id")

		if err := d.Conversations.Delete(r.Context(), id, caller.KeyID); err != nil {
			d.writeConversationError(w, err, "delete conversation")
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "conversation.deleted", ResourceType: "conversation", ResourceID: id,
		})
		w.WriteHeader(http.StatusNoContent)
	}))
}

func (d Deps) writeConversationError(w http.ResponseWriter, err error, operation string) {
	if errors.Is(err, conversation.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "conversation not found"})
		return
	}
	d.Log.Error(operation, "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
