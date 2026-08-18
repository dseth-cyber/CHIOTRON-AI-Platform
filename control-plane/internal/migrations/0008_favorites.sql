-- Favourites: the things a caller has marked to come back to.
--
-- Keyed by actor rather than by user. A conversation belongs to the credential
-- that created it, and until the Identity Service issues JWTs the platform has
-- no notion of a person behind a key (ARCHITECTURE-v1 section 13 item 2). The
-- column is named actor_id so that when identities arrive the meaning widens
-- without the schema changing shape.

CREATE TABLE IF NOT EXISTS favorites (
  id BIGSERIAL PRIMARY KEY,
  actor_id TEXT NOT NULL,
  company_id TEXT,
  -- assistant | conversation | document
  kind TEXT NOT NULL,
  -- The id of the favourited record. It is not a foreign key because the three
  -- kinds live in three tables; a favourite pointing at something deleted is
  -- filtered out on read rather than cascading.
  target_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Favouriting twice is the same as favouriting once.
CREATE UNIQUE INDEX IF NOT EXISTS favorites_identity_idx
  ON favorites (actor_id, kind, target_id);
CREATE INDEX IF NOT EXISTS favorites_actor_idx
  ON favorites (actor_id, created_at DESC);
