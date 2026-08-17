-- Knowledge sources, documents, chunks and embeddings.
--
-- Source ACL metadata follows every chunk (ARCHITECTURE-v1 section 6) so
-- retrieval can filter access in SQL, before any content reaches a model.

-- Credentials gain the two remaining ABAC dimensions. Company was already here;
-- department and a reading ceiling complete the predicate set from section 5.
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS department TEXT;
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS max_classification TEXT NOT NULL DEFAULT 'internal';

CREATE TABLE IF NOT EXISTS knowledge_sources (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  -- How the source is reached: `upload` today, connectors later.
  kind TEXT NOT NULL DEFAULT 'upload',
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  -- Defaults applied to documents that arrive without their own ACL metadata.
  default_classification TEXT NOT NULL DEFAULT 'internal',
  company_id TEXT,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS knowledge_sources_slug_idx
  ON knowledge_sources (slug) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS documents (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id UUID NOT NULL REFERENCES knowledge_sources (id),
  title TEXT NOT NULL,
  -- Where StorageProvider put the bytes. The platform never keeps the only copy
  -- of a document inside the database.
  storage_key TEXT NOT NULL,
  mime_type TEXT NOT NULL,
  byte_size BIGINT NOT NULL DEFAULT 0,
  checksum TEXT NOT NULL,

  -- Source ACL metadata. Every chunk inherits these three.
  classification TEXT NOT NULL,
  company_id TEXT,
  department TEXT,
  owner_id TEXT NOT NULL,

  -- pending -> processing -> ready, or failed with a reason.
  status TEXT NOT NULL DEFAULT 'pending',
  error TEXT,
  chunk_count INTEGER NOT NULL DEFAULT 0,
  ingested_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS documents_company_idx
  ON documents (company_id, created_at DESC) WHERE deleted_at IS NULL;
-- The ingestion worker only ever scans what it has not processed.
CREATE INDEX IF NOT EXISTS documents_pending_idx
  ON documents (created_at) WHERE status = 'pending' AND deleted_at IS NULL;
-- One document per source per checksum: re-uploading the same bytes is a no-op
-- rather than a duplicate corpus entry.
CREATE UNIQUE INDEX IF NOT EXISTS documents_source_checksum_idx
  ON documents (source_id, checksum) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS chunks (
  id BIGSERIAL PRIMARY KEY,
  document_id UUID NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL,
  content TEXT NOT NULL,
  char_count INTEGER NOT NULL DEFAULT 0,

  -- Copied from the document so retrieval filters without a join. Section 6
  -- requires the ACL to travel with the chunk, not merely be reachable from it.
  classification TEXT NOT NULL,
  company_id TEXT,
  department TEXT,

  -- 768 dimensions is nomic-embed-text. Changing the embedding model changes
  -- this width, so it needs a new migration and a re-embedding pass rather than
  -- a configuration flip.
  embedding vector(768),

  -- `simple` rather than `english`: the corpus is multilingual and English
  -- stemming would mangle Thai, Chinese, Japanese and Burmese content.
  content_tsv tsvector GENERATED ALWAYS AS (to_tsvector('simple', content)) STORED,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS chunks_document_ordinal_idx
  ON chunks (document_id, ordinal);
CREATE INDEX IF NOT EXISTS chunks_tsv_idx ON chunks USING gin (content_tsv);
CREATE INDEX IF NOT EXISTS chunks_acl_idx ON chunks (company_id, classification);

-- Cosine distance matches how nomic-embed-text is trained to be compared.
CREATE INDEX IF NOT EXISTS chunks_embedding_idx
  ON chunks USING hnsw (embedding vector_cosine_ops);

-- A default upload source, so a document can be posted without first
-- configuring a connector.
INSERT INTO knowledge_sources (slug, name, kind, created_by)
SELECT 'uploads', 'Manual uploads', 'upload', 'migration'
WHERE NOT EXISTS (SELECT 1 FROM knowledge_sources WHERE slug = 'uploads' AND deleted_at IS NULL);
