package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/ratelimit"
)

// Authenticator verifies presented API keys.
type Authenticator interface {
	// Authenticate returns the caller's identity, plus a server-side reason for
	// any failure. The reason is for the log only.
	Authenticate(ctx context.Context, presented string) (auth.Identity, string, error)
	TouchLastUsed(ctx context.Context, keyID string) error
}

// RateLimiter enforces per-key quota.
type RateLimiter interface {
	Allow(ctx context.Context, subject string, limit int) (ratelimit.Decision, error)
}

// AuditRecorder writes the usage and audit outbox.
type AuditRecorder interface {
	Record(ctx context.Context, event audit.Event)
	RecordUsage(ctx context.Context, usage audit.Usage)
	PendingCounts(ctx context.Context) (auditRows, usageRows int, err error)
}

// guard authenticates a request, checks one scope, enforces company scoping and
// applies the caller's rate limit before handing over.
//
// The backend authorizes every action; nothing downstream may assume the caller
// was already checked (ARCHITECTURE-v1 section 5).
func (d Deps) guard(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		presented, ok := bearerToken(r)
		if !ok {
			unauthorized(w, "missing api key")
			return
		}

		identity, reason, err := d.Auth.Authenticate(r.Context(), presented)
		switch {
		case errors.Is(err, auth.ErrInvalidKey):
			d.Log.Warn("api key rejected", "reason", reason, "path", r.URL.Path)
			unauthorized(w, "invalid api key")
			return
		case err != nil:
			d.Log.Error("authenticate api key", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}

		if !identity.HasScope(scope) {
			d.recordDenied(r, identity, scope, "missing scope")
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing scope " + scope})
			return
		}

		// X-Active-Company is honoured only after the credential is validated and
		// only when it matches what the credential is entitled to.
		if active := strings.TrimSpace(r.Header.Get("X-Active-Company")); active != "" {
			if identity.CompanyID == "" || active != identity.CompanyID {
				d.recordDenied(r, identity, scope, "active company mismatch")
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "active company is not permitted for this key"})
				return
			}
		}

		decision, err := d.Limiter.Allow(r.Context(), identity.KeyID, identity.RateLimitPerMinute)
		if err != nil {
			// Fail closed. The limiter protects the compute plane, and Redis is
			// already a declared readiness dependency, so an outage should shed
			// load rather than silently remove the quota ceiling.
			d.Log.Error("rate limit unavailable", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rate limiter unavailable"})
			return
		}
		writeRateLimitHeaders(w, decision)
		if !decision.Allowed {
			d.recordDenied(r, identity, scope, "rate limit exceeded")
			w.Header().Set("Retry-After", strconv.Itoa(decision.RetryAfter(time.Now())))
			writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			return
		}

		if err := d.Auth.TouchLastUsed(r.Context(), identity.KeyID); err != nil {
			// Activity tracking is not worth failing a request over.
			d.Log.Warn("record api key activity", "keyId", identity.KeyID, "error", err)
		}

		next(w, r.WithContext(auth.WithIdentity(r.Context(), identity)))
	}
}

func (d Deps) recordDenied(r *http.Request, identity auth.Identity, scope, reason string) {
	if d.Audit == nil {
		return
	}
	d.Audit.Record(r.Context(), audit.Event{
		ActorID:      identity.KeyID,
		APIKeyID:     identity.KeyID,
		CompanyID:    identity.CompanyID,
		Action:       "request.denied",
		ResourceType: "endpoint",
		ResourceID:   r.URL.Path,
		Outcome:      audit.OutcomeDenied,
		Metadata:     map[string]any{"reason": reason, "scope": scope},
	})
}

func bearerToken(r *http.Request) (string, bool) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return "", false
	}
	scheme, value, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "bearer") || strings.TrimSpace(value) == "" {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="chiotron-ai"`)
	writeJSON(w, http.StatusUnauthorized, map[string]string{"error": message})
}

func writeRateLimitHeaders(w http.ResponseWriter, decision ratelimit.Decision) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))
}
