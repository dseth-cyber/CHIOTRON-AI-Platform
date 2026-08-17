-- Assistants, conversations and messages.
--
-- Assistant-first selection hides provider details from the user
-- (ARCHITECTURE-v1 section 10): an assistant names the logical model and the
-- instructions to use, so the portal never chooses a model name itself.
-- Conversations and messages are AI-owned records with company context and soft
-- deletion (section 8).

CREATE TABLE IF NOT EXISTS assistants (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  -- Prepended as a system message. It is assistant policy, not user content.
  instructions TEXT NOT NULL DEFAULT '',
  -- A logical model id resolved by the compute registry, never a provider name.
  logical_model TEXT NOT NULL,
  temperature DOUBLE PRECISION,
  max_tokens INTEGER,
  company_id TEXT,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS assistants_slug_idx
  ON assistants (slug) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS assistants_company_idx
  ON assistants (company_id) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS conversations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id TEXT NOT NULL,
  api_key_id UUID,
  company_id TEXT,
  assistant_id UUID REFERENCES assistants (id),
  title TEXT NOT NULL DEFAULT '',
  message_count INTEGER NOT NULL DEFAULT 0,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

-- History is listed most-recent-first for one caller at a time.
CREATE INDEX IF NOT EXISTS conversations_actor_idx
  ON conversations (actor_id, updated_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS messages (
  id BIGSERIAL PRIMARY KEY,
  conversation_id UUID NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
  role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  -- Prompt logging is a policy setting (ARCHITECTURE-v1 section 5). When it is
  -- off the turn is still recorded so the transcript keeps its shape, with the
  -- text omitted and this flag set.
  content_redacted BOOLEAN NOT NULL DEFAULT false,
  prompt_tokens INTEGER NOT NULL DEFAULT 0,
  completion_tokens INTEGER NOT NULL DEFAULT 0,
  model TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS messages_conversation_idx
  ON messages (conversation_id, id);

-- A platform with no assistants cannot answer anything, so ship one. It routes
-- to the `default` logical model, whatever that is configured to be.
INSERT INTO assistants (slug, name, description, instructions, logical_model, temperature, created_by)
SELECT 'general',
       'General assistant',
       'Answers general questions using the platform default model.',
       'You are the CHIOTRON enterprise assistant. Answer concisely and say so when you do not know.',
       'default',
       0.2,
       'migration'
WHERE NOT EXISTS (SELECT 1 FROM assistants WHERE slug = 'general' AND deleted_at IS NULL);
