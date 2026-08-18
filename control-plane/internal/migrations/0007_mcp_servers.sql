-- Registered Model Context Protocol servers.
--
-- MCP tools are reached over the network by a governed client (ARCHITECTURE-v1
-- section 12 item 9). They join the same registry as the built-in tools, so a
-- remote tool is authorized, rate limited and audited exactly like a local one:
-- there is one governance path, not two.

CREATE TABLE IF NOT EXISTS mcp_servers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  base_url TEXT NOT NULL,

  -- The scope a caller must hold for any tool this server exposes. It is set per
  -- server rather than per remote tool: the server decides what tools it offers
  -- and may change them, so the platform cannot let it choose its own
  -- permissions.
  required_scope TEXT NOT NULL DEFAULT 'tools:read',

  -- Credentials for reaching the server. Never returned by any API: a caller
  -- authorized to use a tool is not authorized to learn how the platform
  -- authenticates to it.
  headers JSONB NOT NULL DEFAULT '{}'::jsonb,

  -- An allowlist of remote tool names. Empty means every tool the server
  -- advertises, which is convenient in development and too permissive to be the
  -- default in production.
  allowed_tools TEXT[] NOT NULL DEFAULT '{}',

  company_id TEXT,
  enabled BOOLEAN NOT NULL DEFAULT true,
  max_calls_per_minute INTEGER NOT NULL DEFAULT 30,
  timeout_seconds INTEGER NOT NULL DEFAULT 30,

  -- Discovery state, so an operator can see why a server's tools are missing
  -- without reading logs.
  last_discovered_at TIMESTAMPTZ,
  discovered_tools INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,

  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS mcp_servers_slug_idx
  ON mcp_servers (slug) WHERE deleted_at IS NULL;

-- Remote tool calls are audited in the same table as local ones, so a single
-- query answers "what did this key make the platform do".
ALTER TABLE tool_calls ADD COLUMN IF NOT EXISTS server_slug TEXT;
