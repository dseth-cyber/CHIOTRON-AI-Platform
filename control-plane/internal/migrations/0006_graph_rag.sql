-- Graph nodes, edges and the mentions that link them back to source text.
--
-- GraphRAG starts on AI-owned tables to avoid premature infrastructure
-- (ARCHITECTURE-v1 section 6). A GraphProvider abstraction sits above this, so
-- moving to Neo4j is an adapter change rather than an orchestration change.
--
-- Nodes and edges carry the same ACL triple as chunks. Traversal filters at every
-- hop, not only at the seed: a node the caller may read must not become a bridge
-- to one it may not.

CREATE TABLE IF NOT EXISTS graph_nodes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  -- entity | acronym | document
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  -- Case-folded form used for identity, so "Control Plane" and "control plane"
  -- are one node rather than two.
  normalised_name TEXT NOT NULL,
  properties JSONB NOT NULL DEFAULT '{}'::jsonb,

  classification TEXT NOT NULL,
  company_id TEXT,
  department TEXT,

  mention_count INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- Identity is kind plus normalised name within a tenant. company_id is coalesced
-- because NULL never equals NULL in a unique index, which would let one
-- platform-wide entity be inserted repeatedly.
CREATE UNIQUE INDEX IF NOT EXISTS graph_nodes_identity_idx
  ON graph_nodes (kind, normalised_name, coalesce(company_id, '')) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS graph_nodes_acl_idx
  ON graph_nodes (company_id, classification) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS graph_nodes_name_idx
  ON graph_nodes (normalised_name) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS graph_edges (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  source_id UUID NOT NULL REFERENCES graph_nodes (id) ON DELETE CASCADE,
  target_id UUID NOT NULL REFERENCES graph_nodes (id) ON DELETE CASCADE,
  -- mentioned_in | co_occurs_with
  relation TEXT NOT NULL,
  -- Which document contributed this observation. Edges are per-document so
  -- re-ingesting a document replaces its contribution instead of multiplying
  -- weights, and withdrawing one removes exactly what it added. Traversal sums
  -- across documents to get the relationship's total strength.
  document_id UUID NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
  weight INTEGER NOT NULL DEFAULT 1,
  properties JSONB NOT NULL DEFAULT '{}'::jsonb,

  classification TEXT NOT NULL,
  company_id TEXT,
  department TEXT,

  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS graph_edges_identity_idx
  ON graph_edges (source_id, target_id, relation, document_id);
CREATE INDEX IF NOT EXISTS graph_edges_source_idx ON graph_edges (source_id, weight DESC);
CREATE INDEX IF NOT EXISTS graph_edges_target_idx ON graph_edges (target_id, weight DESC);
CREATE INDEX IF NOT EXISTS graph_edges_document_idx ON graph_edges (document_id);

-- Provenance: which chunk of which document mentioned this node. A relationship
-- with no source link cannot be cited, and an uncitable claim is not an answer.
CREATE TABLE IF NOT EXISTS graph_mentions (
  id BIGSERIAL PRIMARY KEY,
  node_id UUID NOT NULL REFERENCES graph_nodes (id) ON DELETE CASCADE,
  document_id UUID NOT NULL REFERENCES documents (id) ON DELETE CASCADE,
  chunk_ordinal INTEGER NOT NULL,
  occurrences INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS graph_mentions_identity_idx
  ON graph_mentions (node_id, document_id, chunk_ordinal);
CREATE INDEX IF NOT EXISTS graph_mentions_document_idx
  ON graph_mentions (document_id);

-- Traversal is a tool like any other: it declares a scope and is rate limited.
INSERT INTO tools (slug, name, description, kind, required_scope, created_by)
SELECT 'graph.neighbours', 'Graph neighbours',
       'Finds entities related to a term, within the documents the caller may read.',
       'graph.neighbours', 'knowledge:read', 'migration'
WHERE NOT EXISTS (SELECT 1 FROM tools WHERE slug = 'graph.neighbours' AND deleted_at IS NULL);
