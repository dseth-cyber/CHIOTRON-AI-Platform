package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/chiotron/ai-control-plane/internal/audit"
	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/graph"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/storage"
)

// DocumentStore is the corpus the Gateway reads and writes.
type DocumentStore interface {
	Create(ctx context.Context, params knowledge.CreateParams) (knowledge.Document, error)
	List(ctx context.Context, access knowledge.Access, limit int) ([]knowledge.Document, error)
	Get(ctx context.Context, id string, access knowledge.Access) (knowledge.Document, error)
	ListDeleted(ctx context.Context, access knowledge.Access, limit int) ([]knowledge.Document, error)
	Delete(ctx context.Context, id string, access knowledge.Access) (knowledge.Document, error)
	Restore(ctx context.Context, id string, access knowledge.Access) (knowledge.Document, error)
	Purge(ctx context.Context, id string, access knowledge.Access) (knowledge.Document, error)
	Search(ctx context.Context, query string, embedding []float32, access knowledge.Access, limit int) ([]knowledge.Hit, error)
	Stats(ctx context.Context, access knowledge.Access) (map[string]int, error)
}

// GraphStore is the traversal surface the graph routes need.
type GraphStore interface {
	Search(ctx context.Context, term string, access graph.Access, limit int) ([]graph.Node, error)
	Neighbours(ctx context.Context, seeds []string, traversal graph.Traversal, access graph.Access) (graph.Subgraph, error)
	Forget(ctx context.Context, documentID string) error
	Stats(ctx context.Context, access graph.Access) (graph.Stats, error)
}

// Knowledge bundles everything the document routes need.
type Knowledge struct {
	Documents DocumentStore
	Storage   storage.Provider
	Embedder  provider.EmbeddingProvider
	Policy    knowledge.Policy
	Graph     GraphStore
	Traversal graph.Traversal
}

type uploadBody struct {
	SourceSlug string `json:"sourceSlug,omitempty"`
	Title      string `json:"title"`
	MimeType   string `json:"mimeType,omitempty"`
	// ACL metadata is required on upload (ARCHITECTURE-v1 section 7). Only the
	// classification is asked for: company and department come from the
	// credential, so a caller cannot file a document outside its own scope.
	Classification string `json:"classification"`
	Content        string `json:"content"`
}

type searchBody struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// access derives the retrieval predicates from the credential alone.
func (k Knowledge) access(caller auth.Identity) (knowledge.Access, error) {
	readable, err := k.Policy.Readable(caller.MaxClassification)
	if err != nil {
		return knowledge.Access{}, err
	}
	return knowledge.Access{
		CompanyID:       caller.CompanyID,
		Department:      caller.Department,
		Classifications: readable,
	}, nil
}

func registerKnowledge(mux *http.ServeMux, d Deps) {
	if d.Knowledge.Documents == nil || !d.authenticated() {
		return
	}
	k := d.Knowledge

	mux.HandleFunc("POST /api/v1/documents", d.guard(auth.ScopeKnowledgeWrite, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body uploadBody
		// The body carries the document itself, so the limit is the document
		// limit plus room for JSON escaping of the text.
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, int64(d.Config.MaxDocumentBytes)*2))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			// A body cut short by the reader is a size problem, not a syntax one.
			// Reporting it as malformed JSON would send the caller looking in the
			// wrong place.
			var tooLarge *http.MaxBytesError
			if errors.As(err, &tooLarge) {
				writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
					"error": "document exceeds " + strconv.Itoa(d.Config.MaxDocumentBytes) + " bytes",
				})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		title := strings.TrimSpace(body.Title)
		if title == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
			return
		}
		if strings.TrimSpace(body.Content) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content must not be empty"})
			return
		}
		content := []byte(body.Content)
		if len(content) > d.Config.MaxDocumentBytes {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error": "document exceeds " + strconv.Itoa(d.Config.MaxDocumentBytes) + " bytes",
			})
			return
		}

		mimeType := body.MimeType
		if mimeType == "" {
			mimeType = "text/plain"
		}
		if !knowledge.SupportsMime(mimeType) {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]string{
				"error": "unsupported content type " + mimeType +
					" (supported: " + strings.Join(knowledge.SupportedMimeTypes, ", ") + ")",
			})
			return
		}

		// A caller may not file a document above its own clearance: that would
		// let a writer create content it cannot itself retrieve, and hide it from
		// review.
		classification, err := k.Policy.Normalise(body.Classification)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !k.Policy.Allows(caller.MaxClassification, classification) {
			d.recordDenied(r, caller, auth.ScopeKnowledgeWrite, "classification above clearance")
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "cannot file a document classified above this key's clearance",
			})
			return
		}

		sourceSlug := body.SourceSlug
		if sourceSlug == "" {
			sourceSlug = "uploads"
		}

		// The bytes are stored before the row exists, so a crash leaves an
		// orphaned object rather than a document row pointing at nothing.
		object, err := k.Storage.Put(r.Context(), storage.Key(caller.CompanyID, storage.Checksum(content)), content)
		if err != nil {
			d.Log.Error("store document bytes", "error", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not store the document"})
			return
		}

		document, err := k.Documents.Create(r.Context(), knowledge.CreateParams{
			SourceSlug:     sourceSlug,
			Title:          title,
			MimeType:       mimeType,
			StorageKey:     object.Key,
			ByteSize:       object.Size,
			Checksum:       object.Checksum,
			Classification: classification,
			CompanyID:      caller.CompanyID,
			Department:     caller.Department,
			OwnerID:        caller.KeyID,
		})
		if err != nil {
			d.Log.Error("create document", "error", err)
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "document.uploaded", ResourceType: "document", ResourceID: document.ID,
			Metadata: map[string]any{
				"title": document.Title, "classification": document.Classification,
				"bytes": document.ByteSize, "source": document.SourceSlug,
			},
		})
		writeJSON(w, http.StatusAccepted, map[string]any{"document": document})
	}))

	mux.HandleFunc("GET /api/v1/documents", d.guard(auth.ScopeKnowledgeRead, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		access, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}

		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			parsed, parseErr := strconv.Atoi(raw)
			if parseErr != nil || parsed <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "limit must be a positive integer"})
				return
			}
			limit = parsed
		}

		// The trash is the same corpus under the opposite predicate rather than a
		// separate endpoint, so a caller cannot reach withdrawn documents through
		// a route that forgot to apply the ACL.
		trash := r.URL.Query().Get("trash") == "true"
		list := k.Documents.List
		if trash {
			list = k.Documents.ListDeleted
		}

		documents, err := list(r.Context(), access, limit)
		if err != nil {
			d.writeKnowledgeError(w, err, "list documents")
			return
		}
		if documents == nil {
			documents = []knowledge.Document{}
		}
		stats, err := k.Documents.Stats(r.Context(), access)
		if err != nil {
			d.writeKnowledgeError(w, err, "read corpus stats")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"documents": documents,
			"status":    stats,
			"trash":     trash,
			// Tells a client what it is allowed to see, which is what makes an
			// empty result interpretable.
			"readableClassifications": access.Classifications,
		})
	}))

	mux.HandleFunc("GET /api/v1/documents/{id}", d.guard(auth.ScopeKnowledgeRead, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		access, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}

		document, err := k.Documents.Get(r.Context(), r.PathValue("id"), access)
		if err != nil {
			d.writeKnowledgeError(w, err, "read document")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"document": document})
	}))

	mux.HandleFunc("DELETE /api/v1/documents/{id}", d.guard(auth.ScopeKnowledgeWrite, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		access, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}

		document, err := k.Documents.Delete(r.Context(), r.PathValue("id"), access)
		if err != nil {
			d.writeKnowledgeError(w, err, "delete document")
			return
		}

		// The stored bytes stay. Delete is reversible (ARCHITECTURE-v1 section 8):
		// the chunks are gone, so the document stops answering immediately, and
		// the bytes are what a restore rebuilds them from. Removing them here
		// would make the trash a list of things that can never come back.
		//
		// The graph is forgotten, because a withdrawn document must stop
		// answering through its relationships too. Re-ingestion rebuilds them.
		if k.Graph != nil {
			if err := k.Graph.Forget(r.Context(), document.ID); err != nil {
				d.Log.Error("forget document graph", "documentId", document.ID, "error", err)
			}
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "document.deleted", ResourceType: "document", ResourceID: document.ID,
			Metadata: map[string]any{"title": document.Title},
		})
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("POST /api/v1/documents/{id}/restore", d.guard(auth.ScopeKnowledgeWrite, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		access, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}

		document, err := k.Documents.Restore(r.Context(), r.PathValue("id"), access)
		if err != nil {
			d.writeKnowledgeError(w, err, "restore document")
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "document.restored", ResourceType: "document", ResourceID: document.ID,
			Metadata: map[string]any{"title": document.Title},
		})
		// 202, not 200: the row is back but the chunks are not. The worker has to
		// re-ingest before this document answers anything again.
		writeJSON(w, http.StatusAccepted, map[string]any{"document": document})
	}))

	// Permanent removal is a separate verb on a separate path, reachable only for
	// something already in the trash. One click must never destroy anything
	// (ARCHITECTURE-v1 section 8); the confirmation itself is the client's job,
	// but the two-step shape is enforced here.
	mux.HandleFunc("DELETE /api/v1/documents/{id}/purge", d.guard(auth.ScopeKnowledgeWrite, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())
		access, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}

		// The caller has to name what it is destroying. A purge fired at the wrong
		// id by a mis-wired client is refused rather than obeyed.
		if confirm := r.URL.Query().Get("confirm"); confirm != r.PathValue("id") {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "purge requires ?confirm= to repeat the document id",
			})
			return
		}

		document, err := k.Documents.Purge(r.Context(), r.PathValue("id"), access)
		if err != nil {
			d.writeKnowledgeError(w, err, "purge document")
			return
		}

		// The row is gone, so the bytes are now unreferenced and safe to remove.
		if err := k.Storage.Delete(r.Context(), document.StorageKey()); err != nil {
			d.Log.Error("delete stored bytes", "documentId", document.ID, "error", err)
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "document.purged", ResourceType: "document", ResourceID: document.ID,
			Metadata: map[string]any{"title": document.Title, "bytes": document.ByteSize},
		})
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("POST /api/v1/knowledge/search", d.guard(auth.ScopeKnowledgeRead, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		var body searchBody
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		query := strings.TrimSpace(body.Query)
		if query == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query must not be empty"})
			return
		}

		access, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}

		ctx, cancel := contextWithTimeout(r, d.Config.ComputeTimeout)
		defer cancel()

		embeddings, err := k.Embedder.Embed(ctx, []string{query})
		if err != nil {
			// Embedding runs on the compute plane, so its loss is a 503 about
			// VM5 rather than a Control Plane failure.
			d.Log.Error("embed search query", "error", err)
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "embedding provider unavailable"})
			return
		}

		hits, err := k.Documents.Search(ctx, query, embeddings[0], access, body.Limit)
		if err != nil {
			d.writeKnowledgeError(w, err, "search knowledge")
			return
		}
		if hits == nil {
			hits = []knowledge.Hit{}
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "knowledge.searched", ResourceType: "knowledge", Outcome: audit.OutcomeSuccess,
			// The query itself is not recorded: it is user content, which is
			// minimised in logs and audit metadata (section 5).
			Metadata: map[string]any{"hits": len(hits), "queryLength": len([]rune(query))},
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"hits":                    hits,
			"readableClassifications": access.Classifications,
		})
	}))

	if k.Graph == nil {
		return
	}
	// Traversal is read-only over the same corpus, so it needs the same scope as
	// search rather than one of its own.
	mux.HandleFunc("GET /api/v1/graph/neighbours", d.guard(auth.ScopeKnowledgeRead, func(w http.ResponseWriter, r *http.Request) {
		caller, _ := auth.IdentityFrom(r.Context())

		term := strings.TrimSpace(r.URL.Query().Get("term"))
		if term == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "term is required"})
			return
		}
		depth := k.Traversal.Depth
		if raw := r.URL.Query().Get("depth"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "depth must be a positive integer"})
				return
			}
			// The configured depth is a ceiling, not a default a caller may raise.
			depth = min(parsed, k.Traversal.Depth)
		}

		accessSet, err := k.access(caller)
		if err != nil {
			d.writeKnowledgeError(w, err, "derive access")
			return
		}
		graphAccess := graph.Access{
			CompanyID:       accessSet.CompanyID,
			Department:      accessSet.Department,
			Classifications: accessSet.Classifications,
		}

		seeds, err := k.Graph.Search(r.Context(), term, graphAccess, 5)
		if err != nil {
			d.writeKnowledgeError(w, err, "search graph")
			return
		}
		if len(seeds) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"subgraph": graph.Subgraph{}, "readableClassifications": graphAccess.Classifications,
			})
			return
		}

		ids := make([]string, 0, len(seeds))
		for _, seed := range seeds {
			ids = append(ids, seed.ID)
		}
		traversal, err := graph.NewTraversal(depth, k.Traversal.MaxNodes, k.Traversal.Relations)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}

		subgraph, err := k.Graph.Neighbours(r.Context(), ids, traversal, graphAccess)
		if err != nil {
			d.writeKnowledgeError(w, err, "traverse graph")
			return
		}

		d.Audit.Record(r.Context(), audit.Event{
			ActorID: caller.KeyID, APIKeyID: caller.KeyID, CompanyID: caller.CompanyID,
			Action: "graph.traversed", ResourceType: "knowledge",
			// The term is user content, so only its shape is recorded.
			Metadata: map[string]any{
				"seeds": len(subgraph.Seeds), "nodes": len(subgraph.Nodes),
				"edges": len(subgraph.Edges), "depth": depth, "truncated": subgraph.Truncated,
			},
		})
		writeJSON(w, http.StatusOK, map[string]any{
			"subgraph": subgraph, "readableClassifications": graphAccess.Classifications,
		})
	}))
}

func (d Deps) writeKnowledgeError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, knowledge.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
	case errors.Is(err, knowledge.ErrUnknownClassification):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		d.Log.Error(operation, "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
}
