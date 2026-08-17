package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/agent"
	"github.com/chiotron/ai-control-plane/internal/assistant"
	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/conversation"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/tool"
)

// AgentRunStore persists run traces.
type AgentRunStore interface {
	Save(ctx context.Context, caller auth.Identity, assistantID, conversationID,
		question, status, failure string, answer agent.Answer) (string, error)
	Get(ctx context.Context, id, actorID string) (agent.RunSummary, []agent.Step, error)
}

// Agent bundles what the agent routes need.
type Agent struct {
	Orchestrator *agent.Orchestrator
	Runs         AgentRunStore
	Tools        *tool.Registry
}

type answerBody struct {
	Assistant      string   `json:"assistant,omitempty"`
	Question       string   `json:"question"`
	ConversationID string   `json:"conversationId,omitempty"`
	Tools          []string `json:"tools,omitempty"`
}

func registerAgent(mux *http.ServeMux, d Deps) {
	if d.Agent.Orchestrator == nil || d.Assistants == nil || !d.authenticated() {
		return
	}

	// The catalogue is filtered to what the caller may actually call, so a client
	// never offers a tool that would be refused.
	mux.HandleFunc("GET /api/v1/tools", d.guard(auth.ScopeToolsRead, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		registrations := d.Agent.Tools.Available(caller)
		tools := make([]map[string]any, 0, len(registrations))
		for _, registration := range registrations {
			arguments, err := d.Agent.Tools.Describe(caller, registration.Slug)
			if err != nil {
				continue
			}
			tools = append(tools, map[string]any{
				"slug": registration.Slug, "name": registration.Name,
				"description": registration.Description, "requiredScope": registration.RequiredScope,
				"maxCallsPerMinute": registration.MaxCallsPerMinute, "arguments": arguments,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
	}))

	mux.HandleFunc("POST /api/v1/agent/answer", d.guard(auth.ScopeAgentRun, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body answerBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxChatBody))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		question := strings.TrimSpace(body.Question)
		if question == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question must not be empty"})
			return
		}

		reference := body.Assistant
		if reference == "" {
			reference = "general"
		}
		selected, err := d.Assistants.Resolve(r.Context(), reference, caller.CompanyID)
		if err != nil {
			if errors.Is(err, assistant.ErrNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			d.Log.Error("resolve assistant", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		// A conversation is optional, but if named it must belong to the caller.
		conversationID := body.ConversationID
		if conversationID != "" {
			if d.Conversations == nil {
				writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "stored conversations are not available"})
				return
			}
			if _, err := d.Conversations.Get(r.Context(), conversationID, caller.KeyID); err != nil {
				d.writeConversationError(w, err, "read conversation")
				return
			}
		}

		ctx, cancel := contextWithTimeout(r, d.Config.ComputeTimeout)
		defer cancel()

		answer, runErr := d.Agent.Orchestrator.Answer(ctx, agent.Request{
			Caller: caller, Assistant: selected, Question: question,
			ConversationID: conversationID, Tools: body.Tools,
		})

		status, failure := agent.OutcomeSuccess, ""
		if runErr != nil {
			status, failure = agent.OutcomeFailure, runErr.Error()
		}
		// The trace is saved either way: a failed run is exactly what somebody
		// investigating a bad answer needs to see.
		runID, saveErr := d.Agent.Runs.Save(r.Context(), caller, selected.ID, conversationID,
			question, status, failure, answer)
		if saveErr != nil {
			d.Log.Error("save agent run", "error", saveErr)
		}
		answer.RunID = runID

		if runErr != nil {
			d.Audit.RecordUsage(r.Context(), audit.Usage{
				ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
				LogicalModel: selected.LogicalModel, Provider: answer.Provider, Model: answer.Model,
				LatencyMs: answer.LatencyMs, Outcome: audit.OutcomeFailure,
			})
			d.writeChatError(w, runErr, provider.Route{Logical: selected.LogicalModel})
			return
		}

		d.Audit.RecordUsage(r.Context(), audit.Usage{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			LogicalModel: selected.LogicalModel, Provider: answer.Provider, Model: answer.Model,
			PromptTokens:     answer.Usage.PromptTokens,
			CompletionTokens: answer.Usage.CompletionTokens,
			TotalTokens:      answer.Usage.TotalTokens,
			LatencyMs:        answer.LatencyMs,
			Outcome:          audit.OutcomeSuccess,
		})
		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "agent.answered", ResourceType: "agent_run", ResourceID: runID,
			Metadata: map[string]any{
				"assistant": selected.Slug, "citations": len(answer.Citations),
				"grounded": answer.Grounded, "conflicted": answer.Conflicted,
				"steps": len(answer.Steps), "tools": answer.ToolsUsed,
			},
		})

		// The turn is appended to the conversation so an agent answer joins the
		// same transcript a plain chat answer would.
		if conversationID != "" {
			if err := d.Conversations.AppendTurn(r.Context(), conversationID, caller.KeyID, conversation.Turn{
				Question: question, Answer: answer.Content,
				PromptTokens: answer.Usage.PromptTokens, CompletionTokens: answer.Usage.CompletionTokens,
				Model: answer.Model,
			}); err != nil {
				d.Log.Error("record agent turn", "conversationId", conversationID, "error", err)
			}
		}

		writeJSON(w, http.StatusOK, answer)
	}))

	if d.Agent.Runs == nil {
		return
	}
	mux.HandleFunc("GET /api/v1/agent/runs/{id}", d.guard(auth.ScopeAgentRun, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		summary, steps, err := d.Agent.Runs.Get(r.Context(), r.PathValue("id"), caller.KeyID)
		if err != nil {
			if errors.Is(err, agent.ErrRunNotFound) {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "agent run not found"})
				return
			}
			d.Log.Error("read agent run", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if steps == nil {
			steps = []agent.Step{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"run": summary, "steps": steps})
	}))
}
