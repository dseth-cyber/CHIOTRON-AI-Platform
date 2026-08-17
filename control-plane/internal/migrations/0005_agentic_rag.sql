-- Tool registry, agent runs and the tool-call audit trail.
--
-- Every agent and tool execution is authorized by the backend and creates an
-- audit event (ARCHITECTURE-v1 section 5). The run trace exists so an answer can
-- be explained after the fact: which searches ran, which tools were called, and
-- which chunks the answer was built from.

-- Retrieval is assistant policy, not a global switch: one assistant may ground
-- every answer in the corpus while another answers from the model alone.
ALTER TABLE assistants ADD COLUMN IF NOT EXISTS retrieval TEXT NOT NULL DEFAULT 'auto';
ALTER TABLE assistants ADD COLUMN IF NOT EXISTS max_steps INTEGER NOT NULL DEFAULT 3;

CREATE TABLE IF NOT EXISTS tools (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  -- Which built-in implementation backs this registration. An unknown kind is
  -- refused at startup rather than at call time.
  kind TEXT NOT NULL,
  config JSONB NOT NULL DEFAULT '{}'::jsonb,
  -- The scope a caller must hold for the agent to use this tool on its behalf.
  required_scope TEXT NOT NULL,
  company_id TEXT,
  enabled BOOLEAN NOT NULL DEFAULT true,
  max_calls_per_minute INTEGER NOT NULL DEFAULT 30,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS tools_slug_idx
  ON tools (slug) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS agent_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id TEXT NOT NULL,
  api_key_id UUID,
  company_id TEXT,
  assistant_id UUID REFERENCES assistants (id),
  conversation_id UUID REFERENCES conversations (id),
  -- The question itself is user content and is stored only when prompt logging
  -- is on, the same rule conversations follow.
  question TEXT NOT NULL DEFAULT '',
  question_redacted BOOLEAN NOT NULL DEFAULT false,
  status TEXT NOT NULL,
  step_count INTEGER NOT NULL DEFAULT 0,
  citation_count INTEGER NOT NULL DEFAULT 0,
  -- Set when the retrieved sources disagreed enough to be worth flagging.
  conflicted BOOLEAN NOT NULL DEFAULT false,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  error TEXT,
  trace_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS agent_runs_actor_idx
  ON agent_runs (actor_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_steps (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID NOT NULL REFERENCES agent_runs (id) ON DELETE CASCADE,
  ordinal INTEGER NOT NULL,
  -- plan | retrieve | tool | synthesise
  kind TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  detail JSONB NOT NULL DEFAULT '{}'::jsonb,
  outcome TEXT NOT NULL,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_steps_run_ordinal_idx
  ON agent_steps (run_id, ordinal);

CREATE TABLE IF NOT EXISTS tool_calls (
  id BIGSERIAL PRIMARY KEY,
  run_id UUID REFERENCES agent_runs (id) ON DELETE SET NULL,
  tool_slug TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  api_key_id UUID,
  company_id TEXT,
  arguments JSONB NOT NULL DEFAULT '{}'::jsonb,
  outcome TEXT NOT NULL,
  error TEXT,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  trace_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- Drains to ai.tool.execution.v1 once Kafka exists (section 7).
  published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS tool_calls_unpublished_idx
  ON tool_calls (created_at) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS tool_calls_actor_idx
  ON tool_calls (actor_id, created_at DESC);

-- The tools the platform ships with. Each names a built-in implementation and
-- the scope a caller needs before the agent may use it on their behalf.
INSERT INTO tools (slug, name, description, kind, required_scope, created_by)
SELECT 'knowledge.search', 'Knowledge search',
       'Searches the document corpus the caller is allowed to read.',
       'knowledge.search', 'knowledge:read', 'migration'
WHERE NOT EXISTS (SELECT 1 FROM tools WHERE slug = 'knowledge.search' AND deleted_at IS NULL);

INSERT INTO tools (slug, name, description, kind, required_scope, created_by)
SELECT 'compute.health', 'Compute plane status',
       'Reports whether the model providers are reachable and which models are loaded.',
       'compute.health', 'models:read', 'migration'
WHERE NOT EXISTS (SELECT 1 FROM tools WHERE slug = 'compute.health' AND deleted_at IS NULL);

INSERT INTO tools (slug, name, description, kind, required_scope, created_by)
SELECT 'platform.time', 'Platform time',
       'Returns the current platform time in UTC.',
       'platform.time', 'models:read', 'migration'
WHERE NOT EXISTS (SELECT 1 FROM tools WHERE slug = 'platform.time' AND deleted_at IS NULL);
