-- Platform settings and prompt template registry.
-- ARCHITECTURE-v1 & Phase 14: Platform settings and prompt template registry owned
-- by the database.

CREATE TABLE IF NOT EXISTS platform_settings (
  key TEXT PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}'::jsonb,
  description TEXT NOT NULL DEFAULT '',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by TEXT NOT NULL DEFAULT 'system'
);

CREATE TABLE IF NOT EXISTS prompt_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  template TEXT NOT NULL,
  variables JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_by TEXT NOT NULL DEFAULT 'system',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS prompt_templates_slug_idx
  ON prompt_templates (slug) WHERE deleted_at IS NULL;

-- Seed default platform settings
INSERT INTO platform_settings (key, value, description)
VALUES 
  ('default_temperature', '0.7'::jsonb, 'Default model temperature for completions'),
  ('max_document_size_bytes', '10485760'::jsonb, 'Maximum upload size in bytes for knowledge documents')
ON CONFLICT (key) DO NOTHING;

-- Seed initial prompt templates
INSERT INTO prompt_templates (slug, name, description, template, variables)
VALUES
  (
    'default_assistant',
    'Default Assistant Instructions',
    'General purpose system prompt for assistant interactions',
    'You are a helpful enterprise AI assistant. Answer accurately based on the provided context.',
    '["context"]'::jsonb
  ),
  (
    'summarize_brief',
    'Document Summariser',
    'Prompt template for summarizing documents into concise bullet points',
    'Please summarize the following content into key executive bullet points:\n\n{{content}}',
    '["content"]'::jsonb
  )
ON CONFLICT DO NOTHING;
