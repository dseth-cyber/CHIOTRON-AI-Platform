-- API keys, and the usage and audit outbox the Gateway writes to.
--
-- Keys are platform-owned credentials: they are hashed, scoped, rate-limited,
-- expirable and auditable, and the raw value is shown once only
-- (ARCHITECTURE-v1 section 5). This is deliberately separate from the JWTs
-- issued by the existing Identity Service.

CREATE TABLE IF NOT EXISTS api_keys (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  -- The lookup half of the presented key. Stored in the clear so a request can
  -- find its row in one indexed read without hashing every candidate.
  prefix TEXT NOT NULL,
  -- SHA-256 of the secret half. The secret is 256 bits of randomness rather
  -- than a user-chosen password, so a password-hardening KDF would add
  -- per-request latency without adding meaningful resistance.
  secret_hash TEXT NOT NULL,
  scopes TEXT[] NOT NULL DEFAULT '{}',
  company_id TEXT,
  rate_limit_per_minute INTEGER NOT NULL DEFAULT 60,
  created_by TEXT NOT NULL,
  expires_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ,
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS api_keys_prefix_idx
  ON api_keys (prefix) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS api_keys_company_idx
  ON api_keys (company_id) WHERE deleted_at IS NULL;

-- Audit records gain the context needed to answer "who did what, under which
-- company, in which trace" and an outbox marker for fan-out to ai.audit.v1.
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS company_id TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS api_key_id UUID;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS trace_id TEXT;
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS outcome TEXT NOT NULL DEFAULT 'success';
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ;

-- Partial index: the publisher only ever scans what it has not sent yet.
CREATE INDEX IF NOT EXISTS audit_logs_unpublished_idx
  ON audit_logs (created_at) WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS usage_events (
  id BIGSERIAL PRIMARY KEY,
  actor_id TEXT NOT NULL,
  api_key_id UUID,
  company_id TEXT,
  logical_model TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  latency_ms BIGINT NOT NULL DEFAULT 0,
  outcome TEXT NOT NULL,
  trace_id TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS usage_events_unpublished_idx
  ON usage_events (created_at) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS usage_events_company_created_idx
  ON usage_events (company_id, created_at DESC);
