package knowledge

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")

// rrfConstant damps the contribution of low-ranked hits in reciprocal rank
// fusion. 60 is the value the technique was published with.
const rrfConstant = 60

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusReady      = "ready"
	StatusFailed     = "failed"
)

type Document struct {
	ID             string     `json:"id"`
	SourceSlug     string     `json:"sourceSlug"`
	Title          string     `json:"title"`
	MimeType       string     `json:"mimeType"`
	ByteSize       int64      `json:"byteSize"`
	Checksum       string     `json:"checksum"`
	Classification string     `json:"classification"`
	CompanyID      string     `json:"companyId,omitempty"`
	Department     string     `json:"department,omitempty"`
	OwnerID        string     `json:"ownerId"`
	Status         string     `json:"status"`
	Error          string     `json:"error,omitempty"`
	ChunkCount     int        `json:"chunkCount"`
	IngestedAt     *time.Time `json:"ingestedAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`

	// storageKey stays unexported: it is where the bytes live, which is an
	// implementation detail of StorageProvider and not something an API caller
	// should learn.
	storageKey string
}

func (d Document) StorageKey() string { return d.storageKey }

// Access is the ABAC predicate set derived from the caller's credential. It is
// never taken from the request body.
type Access struct {
	CompanyID  string
	Department string
	// Classifications is the explicit allow-list from Policy.Readable.
	Classifications []string
}

type CreateParams struct {
	SourceSlug     string
	Title          string
	MimeType       string
	StorageKey     string
	ByteSize       int64
	Checksum       string
	Classification string
	CompanyID      string
	Department     string
	OwnerID        string
}

// Hit is one retrieved chunk with the provenance a citation needs.
type Hit struct {
	ChunkID        int64   `json:"chunkId"`
	DocumentID     string  `json:"documentId"`
	DocumentTitle  string  `json:"documentTitle"`
	Ordinal        int     `json:"ordinal"`
	Content        string  `json:"content"`
	Classification string  `json:"classification"`
	Score          float64 `json:"score"`
	VectorScore    float64 `json:"vectorScore,omitempty"`
	KeywordScore   float64 `json:"keywordScore,omitempty"`
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const documentColumns = `d.id::text, s.slug, d.title, d.mime_type, d.byte_size, d.checksum,
	d.classification, coalesce(d.company_id, ''), coalesce(d.department, ''), d.owner_id,
	d.status, coalesce(d.error, ''), d.chunk_count, d.ingested_at, d.created_at, d.storage_key`

// Create records an uploaded document as pending ingestion.
//
// Re-uploading identical bytes to the same source returns the existing row: the
// unique index makes that a conflict, and treating it as success keeps an
// idempotent upload from failing a retry.
func (s *Store) Create(ctx context.Context, params CreateParams) (Document, error) {
	row := s.pool.QueryRow(ctx, `
		WITH source AS (
			SELECT id FROM knowledge_sources
			WHERE slug = $1 AND deleted_at IS NULL AND enabled
		), inserted AS (
			INSERT INTO documents (source_id, title, storage_key, mime_type, byte_size, checksum,
			                       classification, company_id, department, owner_id)
			SELECT source.id, $2, $3, $4, $5, $6, $7, nullif($8, ''), nullif($9, ''), $10
			FROM source
			ON CONFLICT DO NOTHING
			RETURNING *
		)
		SELECT `+documentColumns+`
		FROM inserted d JOIN knowledge_sources s ON s.id = d.source_id`,
		params.SourceSlug, params.Title, params.StorageKey, params.MimeType, params.ByteSize,
		params.Checksum, params.Classification, params.CompanyID, params.Department, params.OwnerID)

	document, err := scanDocument(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// Either the source does not exist or the checksum is already stored.
		// Tell them apart so the caller gets a usable message.
		return s.resolveCreateConflict(ctx, params)
	case err != nil:
		return Document{}, fmt.Errorf("create document: %w", err)
	}
	return document, nil
}

func (s *Store) resolveCreateConflict(ctx context.Context, params CreateParams) (Document, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+documentColumns+`
		FROM documents d JOIN knowledge_sources s ON s.id = d.source_id
		WHERE s.slug = $1 AND d.checksum = $2 AND d.deleted_at IS NULL`,
		params.SourceSlug, params.Checksum)

	existing, err := scanDocument(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Document{}, fmt.Errorf("knowledge source %q is not available", params.SourceSlug)
	case err != nil:
		return Document{}, fmt.Errorf("resolve document conflict: %w", err)
	}
	return existing, nil
}

func (s *Store) List(ctx context.Context, access Access, limit int) ([]Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+documentColumns+`
		FROM documents d JOIN knowledge_sources s ON s.id = d.source_id
		WHERE d.deleted_at IS NULL
		  AND (d.company_id IS NULL OR d.company_id = nullif($1, ''))
		  AND (d.department IS NULL OR d.department = nullif($2, ''))
		  AND d.classification = ANY($3)
		ORDER BY d.created_at DESC
		LIMIT $4`, access.CompanyID, access.Department, access.Classifications, limit)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var documents []Document
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("list documents: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	return documents, nil
}

// Get applies the same predicates as List, so a document the caller may not read
// is reported as missing rather than forbidden.
func (s *Store) Get(ctx context.Context, id string, access Access) (Document, error) {
	if !isUUID(id) {
		return Document{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	}
	row := s.pool.QueryRow(ctx, `
		SELECT `+documentColumns+`
		FROM documents d JOIN knowledge_sources s ON s.id = d.source_id
		WHERE d.id = $1::uuid AND d.deleted_at IS NULL
		  AND (d.company_id IS NULL OR d.company_id = nullif($2, ''))
		  AND (d.department IS NULL OR d.department = nullif($3, ''))
		  AND d.classification = ANY($4)`,
		id, access.CompanyID, access.Department, access.Classifications)

	document, err := scanDocument(row)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Document{}, fmt.Errorf("%w: %q", ErrNotFound, id)
	case err != nil:
		return Document{}, fmt.Errorf("read document: %w", err)
	}
	return document, nil
}

// Delete soft-deletes the document and removes its chunks, so retrieval stops
// returning content the operator has withdrawn while the record itself survives
// for audit.
func (s *Store) Delete(ctx context.Context, id string, access Access) (Document, error) {
	document, err := s.Get(ctx, id, access)
	if err != nil {
		return Document{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Document{}, fmt.Errorf("begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, `DELETE FROM chunks WHERE document_id = $1::uuid`, id); err != nil {
		return Document{}, fmt.Errorf("delete chunks: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE documents SET deleted_at = now(), updated_at = now(), chunk_count = 0
		WHERE id = $1::uuid`, id); err != nil {
		return Document{}, fmt.Errorf("delete document: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Document{}, fmt.Errorf("commit delete: %w", err)
	}
	return document, nil
}

// ClaimPending takes ownership of pending documents for one worker pass.
//
// SKIP LOCKED lets several replicas share the queue without coordinating: each
// takes rows nobody else has locked instead of waiting behind them.
func (s *Store) ClaimPending(ctx context.Context, limit int) ([]Document, error) {
	rows, err := s.pool.Query(ctx, `
		WITH claimed AS (
			SELECT id FROM documents
			WHERE status = $1 AND deleted_at IS NULL
			ORDER BY created_at
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		), moved AS (
			UPDATE documents SET status = $3, updated_at = now()
			WHERE id IN (SELECT id FROM claimed)
			RETURNING *
		)
		SELECT `+documentColumns+`
		FROM moved d JOIN knowledge_sources s ON s.id = d.source_id`,
		StatusPending, limit, StatusProcessing)
	if err != nil {
		return nil, fmt.Errorf("claim pending documents: %w", err)
	}
	defer rows.Close()

	var documents []Document
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("claim pending documents: %w", err)
		}
		documents = append(documents, document)
	}
	return documents, rows.Err()
}

// SaveChunks replaces a document's chunks and marks it ready, in one
// transaction: a half-embedded document would answer queries with a partial
// corpus and look healthy doing it.
func (s *Store) SaveChunks(ctx context.Context, document Document, contents []string, embeddings [][]float32) error {
	if len(contents) != len(embeddings) {
		return fmt.Errorf("%d chunks but %d embeddings", len(contents), len(embeddings))
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin save chunks: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	if _, err := tx.Exec(ctx, `DELETE FROM chunks WHERE document_id = $1::uuid`, document.ID); err != nil {
		return fmt.Errorf("clear previous chunks: %w", err)
	}

	for i, content := range contents {
		if _, err := tx.Exec(ctx, `
			INSERT INTO chunks (document_id, ordinal, content, char_count,
			                    classification, company_id, department, embedding)
			VALUES ($1::uuid, $2, $3, $4, $5, nullif($6, ''), nullif($7, ''), $8::vector)`,
			document.ID, i, content, len([]rune(content)),
			document.Classification, document.CompanyID, document.Department,
			vectorLiteral(embeddings[i])); err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE documents
		SET status = $2, chunk_count = $3, error = NULL, ingested_at = now(), updated_at = now()
		WHERE id = $1::uuid`, document.ID, StatusReady, len(contents)); err != nil {
		return fmt.Errorf("mark document ready: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit save chunks: %w", err)
	}
	return nil
}

// MarkFailed records why a document could not be ingested. The reason is stored
// so an operator can fix the input rather than guess.
func (s *Store) MarkFailed(ctx context.Context, id, reason string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE documents SET status = $2, error = $3, updated_at = now()
		WHERE id = $1::uuid`, id, StatusFailed, reason)
	if err != nil {
		return fmt.Errorf("mark document failed: %w", err)
	}
	return nil
}

// Search performs hybrid retrieval: vector similarity fused with keyword rank.
//
// The ACL predicates are repeated inside each candidate query rather than shared
// through a CTE, so the planner can still reach the HNSW and GIN indexes. A
// filter applied after ranking would be both slower and wrong.
func (s *Store) Search(ctx context.Context, query string, embedding []float32, access Access, limit int) ([]Hit, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	// Each side contributes a deeper candidate list than the final result, so
	// fusion has something to reorder.
	candidates := limit * 5

	rows, err := s.pool.Query(ctx, `
		WITH ask AS (
			-- plainto_tsquery ANDs every term, and the 'simple' configuration
			-- strips no stopwords, so a natural-language question such as
			-- "how much VRAM does the GPU have" matches nothing. Rewriting the
			-- operators to OR gives "any of these words", which is what a
			-- retrieval query means; ts_rank still scores how well each
			-- candidate matches.
			SELECT $1::vector AS vec,
			       replace(plainto_tsquery('simple', $2)::text, '&', '|')::tsquery AS tsq
		),
		vector_hits AS (
			SELECT c.id,
			       row_number() OVER (ORDER BY c.embedding <=> ask.vec) AS position,
			       1 - (c.embedding <=> ask.vec) AS score
			FROM chunks c, ask
			WHERE c.embedding IS NOT NULL
			  AND (c.company_id IS NULL OR c.company_id = nullif($3, ''))
			  AND (c.department IS NULL OR c.department = nullif($4, ''))
			  AND c.classification = ANY($5)
			ORDER BY c.embedding <=> ask.vec
			LIMIT $6
		),
		keyword_hits AS (
			SELECT c.id,
			       row_number() OVER (ORDER BY ts_rank(c.content_tsv, ask.tsq) DESC) AS position,
			       ts_rank(c.content_tsv, ask.tsq) AS score
			FROM chunks c, ask
			WHERE c.content_tsv @@ ask.tsq
			  AND (c.company_id IS NULL OR c.company_id = nullif($3, ''))
			  AND (c.department IS NULL OR c.department = nullif($4, ''))
			  AND c.classification = ANY($5)
			ORDER BY ts_rank(c.content_tsv, ask.tsq) DESC
			LIMIT $6
		),
		fused AS (
			SELECT coalesce(v.id, k.id) AS id,
			       coalesce(1.0 / ($7 + v.position), 0) + coalesce(1.0 / ($7 + k.position), 0) AS score,
			       coalesce(v.score, 0) AS vector_score,
			       coalesce(k.score, 0) AS keyword_score
			FROM vector_hits v FULL OUTER JOIN keyword_hits k ON k.id = v.id
		)
		SELECT c.id, d.id::text, d.title, c.ordinal, c.content, c.classification,
		       f.score, f.vector_score, f.keyword_score
		FROM fused f
		JOIN chunks c ON c.id = f.id
		JOIN documents d ON d.id = c.document_id AND d.deleted_at IS NULL
		ORDER BY f.score DESC, c.id
		LIMIT $8`,
		vectorLiteral(embedding), query, access.CompanyID, access.Department,
		access.Classifications, candidates, rrfConstant, limit)
	if err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	defer rows.Close()

	var hits []Hit
	for rows.Next() {
		var hit Hit
		if err := rows.Scan(&hit.ChunkID, &hit.DocumentID, &hit.DocumentTitle, &hit.Ordinal,
			&hit.Content, &hit.Classification, &hit.Score, &hit.VectorScore, &hit.KeywordScore); err != nil {
			return nil, fmt.Errorf("search chunks: %w", err)
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	return hits, nil
}

// Stats reports corpus size for operators and the readiness of ingestion.
func (s *Store) Stats(ctx context.Context, access Access) (map[string]int, error) {
	stats := map[string]int{}
	rows, err := s.pool.Query(ctx, `
		SELECT d.status, count(*)
		FROM documents d
		WHERE d.deleted_at IS NULL
		  AND (d.company_id IS NULL OR d.company_id = nullif($1, ''))
		  AND d.classification = ANY($2)
		GROUP BY d.status`, access.CompanyID, access.Classifications)
	if err != nil {
		return nil, fmt.Errorf("read corpus stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("read corpus stats: %w", err)
		}
		stats[status] = count
	}
	return stats, rows.Err()
}

// vectorLiteral renders a vector for pgvector. pgx has no native codec for the
// type, so it is passed as text and cast in SQL.
func vectorLiteral(embedding []float32) string {
	if len(embedding) == 0 {
		return "[]"
	}
	var builder strings.Builder
	builder.WriteByte('[')
	for i, value := range embedding {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'g', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String()
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, char := range value {
		switch i {
		case 8, 13, 18, 23:
			if char != '-' {
				return false
			}
		default:
			isHex := (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')
			if !isHex {
				return false
			}
		}
	}
	return true
}

type scannable interface{ Scan(dest ...any) error }

func scanDocument(row scannable) (Document, error) {
	var document Document
	err := row.Scan(&document.ID, &document.SourceSlug, &document.Title, &document.MimeType,
		&document.ByteSize, &document.Checksum, &document.Classification, &document.CompanyID,
		&document.Department, &document.OwnerID, &document.Status, &document.Error,
		&document.ChunkCount, &document.IngestedAt, &document.CreatedAt, &document.storageKey)
	return document, err
}
