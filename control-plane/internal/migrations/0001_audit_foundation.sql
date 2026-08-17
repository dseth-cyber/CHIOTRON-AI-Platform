-- Audit log foundation. Every AI request, agent/tool call, SQL execution and
-- configuration change eventually writes here (ARCHITECTURE-v1 section 5).
--
-- The vector extension is infrastructure-owned (infra/postgres/init.sql)
-- because CREATE EXTENSION requires elevated rights the production AI database
-- role is not expected to hold.

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  actor_id TEXT NOT NULL,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS audit_logs_created_at_idx ON audit_logs (created_at DESC);
