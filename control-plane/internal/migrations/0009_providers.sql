-- Compute providers and the model routing table, owned by the platform rather
-- than by the environment.
--
-- ARCHITECTURE-v1 sections 46 and 53: model routing is configuration, not a
-- constant, and it has to be changeable from the Admin UI. Holding it in env
-- vars means every routing change is a redeploy, and a deployment cannot add a
-- provider without an engineer.

CREATE TABLE IF NOT EXISTS providers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',

  -- Which adapter serves this provider. The set is closed: an unknown kind has
  -- no code behind it, so it is refused when the row is written rather than
  -- failing on somebody's first request.
  kind TEXT NOT NULL,
  base_url TEXT NOT NULL,

  -- The API credential, sealed with AES-GCM. Never returned by any endpoint: a
  -- caller authorized to use a model is not authorized to learn how the
  -- platform pays for it.
  credential BYTEA,
  -- The last few characters, so an operator can tell two keys apart in the UI
  -- without the platform ever handing back the secret.
  credential_hint TEXT NOT NULL DEFAULT '',

  -- The most sensitive classification whose content may be sent to this
  -- provider. Public is the default because the safe direction has to be the
  -- one nobody has to remember: an external provider added in a hurry must not
  -- silently become a way for restricted documents to leave the building.
  max_classification TEXT NOT NULL DEFAULT 'public',

  enabled BOOLEAN NOT NULL DEFAULT true,
  timeout_seconds INTEGER NOT NULL DEFAULT 60,
  company_id TEXT,

  -- Connection state from the last check, so an operator sees why a provider's
  -- models are missing without reading logs.
  last_checked_at TIMESTAMPTZ,
  last_status TEXT NOT NULL DEFAULT '',
  last_error TEXT NOT NULL DEFAULT '',

  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS providers_slug_idx
  ON providers (slug) WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS model_routes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

  -- What callers ask for. Upstream names and provider URLs stay on this side of
  -- the boundary so business logic never names a vendor model.
  logical TEXT NOT NULL,
  provider_slug TEXT NOT NULL,
  upstream_model TEXT NOT NULL,

  is_default BOOLEAN NOT NULL DEFAULT false,
  enabled BOOLEAN NOT NULL DEFAULT true,
  company_id TEXT,

  created_by TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS model_routes_logical_idx
  ON model_routes (logical) WHERE deleted_at IS NULL;

-- Exactly one default route. Enforced by the database rather than by the
-- handler: two defaults would make which model answers depend on row order.
CREATE UNIQUE INDEX IF NOT EXISTS model_routes_one_default_idx
  ON model_routes ((is_default)) WHERE is_default AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS model_routes_provider_idx
  ON model_routes (provider_slug) WHERE deleted_at IS NULL;
