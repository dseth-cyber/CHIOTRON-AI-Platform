-- Infrastructure-owned bootstrap. This runs only on an empty data volume.
--
-- Only privileged setup belongs here. Tables are owned by the Control Plane
-- and applied from control-plane/internal/migrations at startup
-- (ARCHITECTURE-v1 section 8), so schema changes reach an existing database
-- instead of silently requiring a volume wipe.

CREATE EXTENSION IF NOT EXISTS vector;
