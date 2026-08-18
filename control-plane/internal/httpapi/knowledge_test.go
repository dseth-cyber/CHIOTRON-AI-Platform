package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/chiotron/ai-control-plane/internal/auth"
	"github.com/chiotron/ai-control-plane/internal/knowledge"
	"github.com/chiotron/ai-control-plane/internal/storage"
)

type fakeDocuments struct {
	created  knowledge.CreateParams
	record   knowledge.Document
	list     []knowledge.Document
	hits     []knowledge.Hit
	err      error
	deleted  string
	restored string
	purged   string
	searched knowledge.Access
	listed   knowledge.Access
	// trash is what ListDeleted returns, and listedTrash records that it was the
	// listing the handler chose.
	trash       []knowledge.Document
	listedTrash bool
}

func (f *fakeDocuments) Create(_ context.Context, params knowledge.CreateParams) (knowledge.Document, error) {
	f.created = params
	if f.err != nil {
		return knowledge.Document{}, f.err
	}
	return f.record, nil
}

func (f *fakeDocuments) List(_ context.Context, access knowledge.Access, _ int) ([]knowledge.Document, error) {
	f.listed = access
	return f.list, f.err
}

func (f *fakeDocuments) Get(_ context.Context, _ string, access knowledge.Access) (knowledge.Document, error) {
	f.listed = access
	if f.err != nil {
		return knowledge.Document{}, f.err
	}
	return f.record, nil
}

func (f *fakeDocuments) ListDeleted(_ context.Context, access knowledge.Access, _ int) ([]knowledge.Document, error) {
	f.listed = access
	f.listedTrash = true
	return f.trash, f.err
}

func (f *fakeDocuments) Delete(_ context.Context, id string, access knowledge.Access) (knowledge.Document, error) {
	f.listed = access
	if f.err != nil {
		return knowledge.Document{}, f.err
	}
	f.deleted = id
	return f.record, nil
}

func (f *fakeDocuments) Restore(_ context.Context, id string, access knowledge.Access) (knowledge.Document, error) {
	f.listed = access
	if f.err != nil {
		return knowledge.Document{}, f.err
	}
	f.restored = id
	return f.record, nil
}

func (f *fakeDocuments) Purge(_ context.Context, id string, access knowledge.Access) (knowledge.Document, error) {
	f.listed = access
	if f.err != nil {
		return knowledge.Document{}, f.err
	}
	f.purged = id
	return f.record, nil
}

func (f *fakeDocuments) Search(_ context.Context, _ string, _ []float32, access knowledge.Access, _ int) ([]knowledge.Hit, error) {
	f.searched = access
	return f.hits, f.err
}

func (f *fakeDocuments) Stats(context.Context, knowledge.Access) (map[string]int, error) {
	return map[string]int{"ready": len(f.list)}, nil
}

type fakeEmbedder struct {
	err    error
	inputs []string
}

func (f *fakeEmbedder) Name() string    { return "fake" }
func (f *fakeEmbedder) Model() string   { return "fake-embed" }
func (f *fakeEmbedder) Dimensions() int { return 3 }

func (f *fakeEmbedder) Embed(_ context.Context, inputs []string) ([][]float32, error) {
	f.inputs = append(f.inputs, inputs...)
	if f.err != nil {
		return nil, f.err
	}
	vectors := make([][]float32, len(inputs))
	for i := range inputs {
		vectors[i] = []float32{0.1, 0.2, 0.3}
	}
	return vectors, nil
}

// spyStorage wraps a real provider and records deletions.
//
// The fake document store cannot produce a document carrying a storage key —
// the field is unexported, which is deliberate — so what these tests can check
// is whether the handler reached for storage at all, which is exactly the
// behaviour that distinguishes a soft delete from a purge.
type spyStorage struct {
	storage.Provider
	deletes []string
}

func (s *spyStorage) Delete(ctx context.Context, key string) error {
	s.deletes = append(s.deletes, key)
	return s.Provider.Delete(ctx, key)
}

type knowledgeFixture struct {
	handler   http.Handler
	documents *fakeDocuments
	embedder  *fakeEmbedder
	audit     *fakeAudit
	storage   *spyStorage
}

func newKnowledgeFixture(t *testing.T, mutate ...func(*Deps)) knowledgeFixture {
	t.Helper()
	policy, err := knowledge.NewPolicy([]string{"public", "internal", "confidential", "restricted"})
	if err != nil {
		t.Fatalf("NewPolicy() returned error: %v", err)
	}
	local, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocal() returned error: %v", err)
	}
	objects := &spyStorage{Provider: local}

	documents := &fakeDocuments{record: sampleDocument()}
	embedder := &fakeEmbedder{}
	recorder := &fakeAudit{}

	identity := fullyScopedIdentity()
	identity.MaxClassification = "confidential"

	deps := Deps{
		Config:  testConfig(),
		Log:     quietLogger(),
		Auth:    &fakeAuthenticator{identity: identity},
		Limiter: allowingLimiter(),
		Audit:   recorder,
		Knowledge: Knowledge{
			Documents: documents,
			Storage:   objects,
			Embedder:  embedder,
			Policy:    policy,
		},
	}
	for _, apply := range mutate {
		apply(&deps)
	}
	return knowledgeFixture{
		handler: NewRouter(deps), documents: documents, embedder: embedder,
		audit: recorder, storage: objects,
	}
}

func sampleDocument() knowledge.Document {
	return knowledge.Document{
		ID: "dddddddd-dddd-dddd-dddd-dddddddddddd", SourceSlug: "uploads",
		Title: "Runbook", MimeType: "text/markdown", Classification: "internal",
		Status: knowledge.StatusPending,
	}
}

func withClearance(level string, scopes ...string) func(*Deps) {
	return func(d *Deps) {
		identity := auth.Identity{
			KeyID: "k", RateLimitPerMinute: 60, MaxClassification: level,
			Scopes: scopes,
		}
		if len(scopes) == 0 {
			identity.Scopes = auth.KnownScopes
		}
		d.Auth = &fakeAuthenticator{identity: identity}
	}
}

func TestUploadStoresBytesAndQueuesIngestion(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	rec := authedPost(t, fixture.handler, "/api/v1/documents",
		`{"title":"Runbook","mimeType":"text/markdown","classification":"internal","content":"# Runbook\n\nRestart the service."}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 (%s)", rec.Code, rec.Body.String())
	}

	// Company and department come from the credential, never the body.
	if fixture.documents.created.OwnerID != fullyScopedIdentity().KeyID {
		t.Errorf("ownerId = %q, want the calling key", fixture.documents.created.OwnerID)
	}
	if fixture.documents.created.Classification != "internal" {
		t.Errorf("classification = %q, want internal", fixture.documents.created.Classification)
	}
	if fixture.documents.created.Checksum == "" || fixture.documents.created.StorageKey == "" {
		t.Errorf("document was created without storage details: %+v", fixture.documents.created)
	}
	if event := fixture.audit.lastEvent(t); event.Action != "document.uploaded" {
		t.Errorf("audit action = %q, want document.uploaded", event.Action)
	}
}

// A writer must not be able to file content it cannot itself retrieve: that
// would hide a document from the person who created it and from review.
func TestUploadRejectsClassificationAboveClearance(t *testing.T) {
	fixture := newKnowledgeFixture(t, withClearance("internal"))

	rec := authedPost(t, fixture.handler, "/api/v1/documents",
		`{"title":"Secret","classification":"restricted","content":"hidden"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (%s)", rec.Code, rec.Body.String())
	}
	if fixture.documents.created.Title != "" {
		t.Error("the document was created despite the refusal")
	}
}

func TestUploadRejectsBadInput(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	cases := map[string]struct {
		body string
		want int
	}{
		"no title":               {`{"classification":"internal","content":"x"}`, http.StatusBadRequest},
		"no content":             {`{"title":"t","classification":"internal","content":"  "}`, http.StatusBadRequest},
		"unknown classification": {`{"title":"t","classification":"cosmic","content":"x"}`, http.StatusBadRequest},
		"unsupported type":       {`{"title":"t","mimeType":"application/pdf","classification":"internal","content":"x"}`, http.StatusUnsupportedMediaType},
		"unknown field":          {`{"title":"t","classification":"internal","content":"x","tags":[]}`, http.StatusBadRequest},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if rec := authedPost(t, fixture.handler, "/api/v1/documents", tc.body); rec.Code != tc.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestUploadRejectsOversizedDocument(t *testing.T) {
	fixture := newKnowledgeFixture(t, func(d *Deps) {
		cfg := testConfig()
		cfg.MaxDocumentBytes = 32
		d.Config = cfg
	})

	body := `{"title":"t","classification":"internal","content":"` + strings.Repeat("x", 200) + `"}`
	if rec := authedPost(t, fixture.handler, "/api/v1/documents", body); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want 413 (%s)", rec.Code, rec.Body.String())
	}
}

// The retrieval predicates come from the credential's clearance, not the request.
func TestSearchFiltersByClearance(t *testing.T) {
	fixture := newKnowledgeFixture(t, withClearance("internal"))

	rec := authedPost(t, fixture.handler, "/api/v1/knowledge/search", `{"query":"restart"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}

	want := []string{"public", "internal"}
	got := fixture.documents.searched.Classifications
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("search was scoped to %v, want %v", got, want)
	}
	if body := decode(t, rec); body["readableClassifications"] == nil {
		t.Error("response does not tell the caller what it may see")
	}
}

func TestSearchEmbedsTheQuery(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	authedPost(t, fixture.handler, "/api/v1/knowledge/search", `{"query":"how do I restart"}`)
	if len(fixture.embedder.inputs) != 1 || fixture.embedder.inputs[0] != "how do I restart" {
		t.Errorf("embedder saw %v, want the query", fixture.embedder.inputs)
	}
}

// The query is user content, which is minimised in audit metadata.
func TestSearchDoesNotRecordTheQueryText(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	authedPost(t, fixture.handler, "/api/v1/knowledge/search", `{"query":"salary of the chief executive"}`)
	event := fixture.audit.lastEvent(t)
	if event.Action != "knowledge.searched" {
		t.Fatalf("audit action = %q, want knowledge.searched", event.Action)
	}
	for key, value := range event.Metadata {
		if text, ok := value.(string); ok && strings.Contains(text, "salary") {
			t.Errorf("audit metadata %q leaked the query: %v", key, value)
		}
	}
}

// Embedding runs on the compute plane, so its loss is a 503 about VM5 rather
// than a Control Plane failure.
func TestSearchReportsEmbeddingOutageAs503(t *testing.T) {
	fixture := newKnowledgeFixture(t, func(d *Deps) {
		d.Knowledge.Embedder = &fakeEmbedder{err: errStorage}
	})

	rec := authedPost(t, fixture.handler, "/api/v1/knowledge/search", `{"query":"x"}`)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestSearchRejectsEmptyQuery(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	if rec := authedPost(t, fixture.handler, "/api/v1/knowledge/search", `{"query":"   "}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestListDocumentsAppliesClearance(t *testing.T) {
	fixture := newKnowledgeFixture(t, withClearance("public"))

	rec := authedGet(t, fixture.handler, "/api/v1/documents")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if got := fixture.documents.listed.Classifications; len(got) != 1 || got[0] != "public" {
		t.Errorf("listing was scoped to %v, want only public", got)
	}
}

func TestDeleteDocument(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	rec := authedDelete(t, fixture.handler, "/api/v1/documents/"+sampleDocument().ID)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (%s)", rec.Code, rec.Body.String())
	}
	if fixture.documents.deleted != sampleDocument().ID {
		t.Errorf("deleted %q, want the id from the path", fixture.documents.deleted)
	}
	if event := fixture.audit.lastEvent(t); event.Action != "document.deleted" {
		t.Errorf("audit action = %q, want document.deleted", event.Action)
	}
}

func TestDocumentRoutesEnforceScopes(t *testing.T) {
	readOnly := newKnowledgeFixture(t, withClearance("confidential", auth.ScopeKnowledgeRead))

	if rec := authedGet(t, readOnly.handler, "/api/v1/documents"); rec.Code != http.StatusOK {
		t.Errorf("GET with knowledge:read = %d, want 200", rec.Code)
	}
	rec := authedPost(t, readOnly.handler, "/api/v1/documents",
		`{"title":"t","classification":"internal","content":"x"}`)
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST without knowledge:write = %d, want 403", rec.Code)
	}

	writeOnly := newKnowledgeFixture(t, withClearance("confidential", auth.ScopeKnowledgeWrite))
	if rec := authedPost(t, writeOnly.handler, "/api/v1/knowledge/search", `{"query":"x"}`); rec.Code != http.StatusForbidden {
		t.Errorf("search without knowledge:read = %d, want 403", rec.Code)
	}
}

func TestKnowledgeRoutesRequireACredential(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	if rec := authedGet(t, fixture.handler, "/api/v1/documents"); rec.Code == http.StatusUnauthorized {
		t.Fatal("the fixture is not authenticating at all")
	}
	if rec := get(t, fixture.handler, "/api/v1/documents", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET without a key = %d, want 401", rec.Code)
	}
}

// The trash lifecycle. Delete has to be reversible (ARCHITECTURE-v1 section 8),
// which means the bytes a restore rebuilds from must survive it.

func TestDeleteKeepsTheStoredBytesSoRestoreCanRebuild(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	rec := authedDelete(t, fixture.handler, "/api/v1/documents/"+sampleDocument().ID)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	// A deleted document that has lost its bytes can never come back, which would
	// make the trash a list of things that only look recoverable.
	if len(fixture.storage.deletes) != 0 {
		t.Fatalf("a soft delete removed stored bytes: %v", fixture.storage.deletes)
	}
}

func TestListReadsTheTrashWhenAsked(t *testing.T) {
	fixture := newKnowledgeFixture(t)
	fixture.documents.trash = []knowledge.Document{sampleDocument()}

	rec := authedGet(t, fixture.handler, "/api/v1/documents?trash=true")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET ?trash=true = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !fixture.documents.listedTrash {
		t.Fatal("?trash=true did not reach ListDeleted")
	}
	// The response says which listing it is, so a client cannot render withdrawn
	// documents as if they were live.
	if !strings.Contains(rec.Body.String(), `"trash":true`) {
		t.Fatalf("response does not declare the trash listing: %s", rec.Body.String())
	}
}

func TestTrashListingStillAppliesTheCallersClearance(t *testing.T) {
	fixture := newKnowledgeFixture(t, withClearance("public"))
	fixture.documents.trash = []knowledge.Document{sampleDocument()}

	if rec := authedGet(t, fixture.handler, "/api/v1/documents?trash=true"); rec.Code != http.StatusOK {
		t.Fatalf("GET ?trash=true = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	// Being in the trash must not widen what a caller may read.
	for _, level := range fixture.documents.listed.Classifications {
		if level != "public" {
			t.Fatalf("trash listing read %v, but this key may only read public", fixture.documents.listed.Classifications)
		}
	}
}

func TestRestoreRequeuesIngestion(t *testing.T) {
	fixture := newKnowledgeFixture(t)

	rec := authedPost(t, fixture.handler, "/api/v1/documents/"+sampleDocument().ID+"/restore", "")
	// 202 rather than 200: the row is back but the chunks are not, so the caller
	// must not treat a restored document as searchable yet.
	if rec.Code != http.StatusAccepted {
		t.Fatalf("restore = %d, want 202: %s", rec.Code, rec.Body.String())
	}
	if fixture.documents.restored != sampleDocument().ID {
		t.Fatalf("restored %q, want the requested id", fixture.documents.restored)
	}
	if action := fixture.audit.lastEvent(t).Action; action != "document.restored" {
		t.Fatalf("audit action = %q, want document.restored", action)
	}
}

func TestPurgeRefusesWithoutAConfirmation(t *testing.T) {
	fixture := newKnowledgeFixture(t)
	id := sampleDocument().ID

	rec := authedDelete(t, fixture.handler, "/api/v1/documents/"+id+"/purge")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("purge without confirm = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if fixture.documents.purged != "" {
		t.Fatal("an unconfirmed purge destroyed the document anyway")
	}

	rec = authedDelete(t, fixture.handler, "/api/v1/documents/"+id+"/purge?confirm=wrong-id")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("purge with a mismatched confirm = %d, want 400", rec.Code)
	}
	if fixture.documents.purged != "" {
		t.Fatal("a mismatched confirmation destroyed the document anyway")
	}
}

func TestPurgeRemovesTheBytesOnceConfirmed(t *testing.T) {
	fixture := newKnowledgeFixture(t)
	id := sampleDocument().ID

	rec := authedDelete(t, fixture.handler, "/api/v1/documents/"+id+"/purge?confirm="+id)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("confirmed purge = %d, want 204: %s", rec.Code, rec.Body.String())
	}
	if fixture.documents.purged != id {
		t.Fatalf("purged %q, want %q", fixture.documents.purged, id)
	}
	if len(fixture.storage.deletes) != 1 {
		t.Fatalf("purge made %d storage deletions, want exactly 1", len(fixture.storage.deletes))
	}
	if action := fixture.audit.lastEvent(t).Action; action != "document.purged" {
		t.Fatalf("audit action = %q, want document.purged", action)
	}
}
