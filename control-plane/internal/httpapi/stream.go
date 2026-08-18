package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/chiotron/ai-control-plane/internal/agent"
	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/provider"
)

// sseTerminator ends the stream. The literal is the convention clients already
// implement, so an off-the-shelf SSE consumer needs no special casing.
const sseTerminator = "[DONE]"

// streamGrace is how much longer than the compute timeout the connection is
// allowed to live, so a slow final chunk is not cut off by the write deadline.
const streamGrace = 30 * time.Second

// streamChat answers with Server-Sent Events.
//
// Response headers are written on the first chunk rather than up front: until
// something has actually been produced, a failure can still be reported with a
// meaningful HTTP status instead of an error buried inside a 200 stream.
func (d Deps) streamChat(ctx context.Context, w http.ResponseWriter, r *http.Request,
	caller auth.Identity, plan chatPlan) {

	controller := http.NewResponseController(w)
	// The server's WriteTimeout is sized for ordinary requests and would cut a
	// long generation short.
	if err := controller.SetWriteDeadline(time.Now().Add(d.Config.ComputeTimeout + streamGrace)); err != nil {
		d.Log.Warn("extend write deadline for stream", "error", err)
	}

	started := time.Now()
	headersSent := false
	sendHeaders := func() {
		if headersSent {
			return
		}
		headersSent = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Nginx buffers proxied responses by default, which would hold the
		// whole stream until completion.
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		_ = controller.Flush()

		// A stored conversation tells the client its id before any content, so a
		// new conversation can be tracked even if the stream later fails.
		if plan.stateful() {
			_ = writeSSE(w, "", map[string]string{
				"conversationId": plan.conversation.ID,
				"assistant":      plan.assistant.Slug,
			})
			_ = controller.Flush()
		}
	}

	emit := func(chunk provider.Chunk) error {
		// A cancelled request means the client is gone; stop pulling from the
		// provider rather than generating tokens nobody will read.
		if err := ctx.Err(); err != nil {
			return err
		}
		sendHeaders()
		if err := writeSSE(w, "", chunk); err != nil {
			return err
		}
		return controller.Flush()
	}

	response, route, err := d.Compute.ChatStream(ctx, plan.logicalModel, plan.request, emit)
	if err != nil {
		d.Audit.RecordUsage(r.Context(), audit.Usage{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			LogicalModel: orDefault(route.Logical, plan.logicalModel), Provider: route.Provider, Model: route.Model,
			LatencyMs: time.Since(started).Milliseconds(), Outcome: audit.OutcomeFailure,
		})

		if !headersSent {
			// Nothing has been committed yet, so the real status can still be sent.
			d.writeChatError(w, err, route)
			return
		}
		// The client already has a 200 and some content. All that is left is to
		// tell it the stream is broken.
		d.Log.Error("compute stream failed", "provider", route.Provider, "model", route.Model, "error", err)
		if writeErr := writeSSE(w, "error", map[string]string{"error": "stream interrupted"}); writeErr == nil {
			_ = controller.Flush()
		}
		return
	}

	d.recordCompletion(r.Context(), caller, plan, route, response)

	// An empty completion still gets a well-formed stream.
	sendHeaders()

	usage := response.Usage
	_ = writeSSE(w, "", provider.Chunk{
		Done:         true,
		FinishReason: response.FinishReason,
		Usage:        &usage,
	})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", sseTerminator)
	_ = controller.Flush()
}

// writeChatError maps a provider failure onto an HTTP status. It is shared by
// the streaming and non-streaming paths so both report an outage the same way.
func (d Deps) writeChatError(w http.ResponseWriter, err error, route provider.Route) {
	switch {
	case errors.Is(err, provider.ErrUnknownModel):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	case errors.Is(err, agent.ErrEgressRefused):
		// A policy refusal, not an outage. Reporting it as a bad gateway would
		// send an operator to debug a provider that is working perfectly, and the
		// caller needs the reason to know it is not going to succeed on a retry.
		d.Log.Warn("egress refused", "provider", route.Provider, "error", err)
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, provider.ErrUnavailable):
		// The Control Plane is fine; the compute plane is not.
		d.Log.Error("compute call failed", "provider", route.Provider, "model", route.Model, "error", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "compute provider unavailable"})
	case errors.Is(err, context.Canceled):
		// The client hung up before we finished; there is nobody to answer.
		d.Log.Info("client cancelled compute call", "provider", route.Provider, "model", route.Model)
	default:
		d.Log.Error("compute call failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "compute call failed"})
	}
}

func writeSSE(w http.ResponseWriter, event string, payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode sse payload: %w", err)
	}
	if event != "" {
		if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", encoded)
	return err
}
