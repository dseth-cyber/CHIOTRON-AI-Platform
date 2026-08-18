package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/favorite"
)

// FavoriteStore holds a caller's marks.
type FavoriteStore interface {
	Add(ctx context.Context, actorID, companyID, kind, targetID string) error
	Remove(ctx context.Context, actorID, kind, targetID string) error
	List(ctx context.Context, actorID, companyID string, classifications []string) ([]favorite.Favorite, error)
	IDs(ctx context.Context, actorID, kind string) ([]string, error)
}

type favoriteBody struct {
	Kind     string `json:"kind"`
	TargetID string `json:"targetId"`
}

// registerFavorites adds the favourites surface.
//
// It is gated by chat:completions for the same reason conversation history is:
// a caller's own marks are part of using the platform, not a separate capability,
// and they are never visible to another credential.
func registerFavorites(mux *http.ServeMux, d Deps) {
	if d.Favorites == nil || !d.authenticated() {
		return
	}

	mux.HandleFunc("GET /api/v1/favorites", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		// Resolving favourites reads documents, so the caller's clearance has to
		// come along: a mark must not become a way to read a title whose
		// classification has since been raised.
		readable, err := d.Knowledge.Policy.Readable(caller.MaxClassification)
		if err != nil {
			// Without a classification policy only the kinds that do not need one
			// can be resolved, which is still useful.
			readable = nil
		}

		favourites, err := d.Favorites.List(r.Context(), caller.KeyID, caller.CompanyID, readable)
		if err != nil {
			d.Log.Error("list favourites", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if favourites == nil {
			favourites = []favorite.Favorite{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"favorites": favourites})
	}))

	mux.HandleFunc("PUT /api/v1/favorites", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, body, ok := decodeFavorite(w, r)
		if !ok {
			return
		}
		if err := d.Favorites.Add(r.Context(), caller.KeyID, caller.CompanyID, body.Kind, body.TargetID); err != nil {
			d.writeFavoriteError(w, err, "add favourite")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("DELETE /api/v1/favorites", d.guard(auth.ScopeChatCompletion, func(w http.ResponseWriter, r *http.Request) {
		caller, body, ok := decodeFavorite(w, r)
		if !ok {
			return
		}
		if err := d.Favorites.Remove(r.Context(), caller.KeyID, body.Kind, body.TargetID); err != nil {
			d.writeFavoriteError(w, err, "remove favourite")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

func decodeFavorite(w http.ResponseWriter, r *http.Request) (auth.Identity, favoriteBody, bool) {
	caller, _ := auth.IdentityFrom(r.Context())

	var body favoriteBody
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return caller, body, false
	}
	if body.TargetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "targetId is required"})
		return caller, body, false
	}
	return caller, body, true
}

func (d Deps) writeFavoriteError(w http.ResponseWriter, err error, operation string) {
	if errors.Is(err, favorite.ErrUnknownKind) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	d.Log.Error(operation, "error", err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}
