# Operational runbooks

Every alert rule in `infra/prometheus/alerts.yml` points at a section here. An
alert nobody knows how to act on is noise, so a rule without a runbook entry
should not be added.

Backup and disaster recovery are at the end. The restore procedure has been
exercised against this platform, and the section says what was actually
verified and what was not.

---

## Control Plane unreachable

**Alert:** `ControlPlaneDown` — `up{job="control-plane"} == 0` for 2 minutes.

The Control Plane is not answering scrapes. ERP is unaffected: an AI outage must
never affect ERP business operations (ARCHITECTURE-v1 section 1).

1. `docker compose ps` — is the container running or restarting?
2. `docker logs chiotron-ai-api-1 --tail 50`. Startup failures are explicit:
   invalid configuration reports **every** problem at once, and a migration
   whose checksum changed refuses to start rather than applying it.
3. `curl -s localhost:8080/healthz` — liveness answers without touching any
   dependency. If this responds while `/readyz` does not, the process is fine
   and a backing service is not.
4. `curl -s localhost:8080/readyz | jq` names the failing dependency and the
   underlying error.

Liveness deliberately ignores dependencies, so restarting the container does not
fix a database outage and should not be the first move.

## Compute plane unavailable

**Alert:** `ComputePlaneUnreachable` — the GPU exporter stopped answering.

Losing VM5 degrades model calls only. `/readyz` stays `200`, retrieval keeps
working, and chat returns `503` with `compute provider unavailable`.

1. `docker ps --filter name=ollama` — the compute profile is not part of a plain
   `docker compose up`, so a routine rebuild can silently leave it out.
2. Bring it back: `docker compose --profile compute up -d ollama gpu-exporter`.
3. `curl -s localhost:8080/api/v1/compute/health -H "Authorization: Bearer $KEY"`
   reports each provider with the underlying reason.

## GPU memory exhausted

**Alert:** `GPUMemoryExhausted` — VRAM above 90% for 10 minutes.

1. `docker exec chiotron-ai-ollama-1 nvidia-smi` shows what holds the memory.
2. Unload idle models: `docker exec chiotron-ai-ollama-1 ollama ps`, then
   `ollama stop <model>`.
3. If this recurs, the model is too large for the card. The development GPU is a
   Quadro P620 with 2 GB and is adequate for smoke tests only — sizing enterprise
   capacity from it is explicitly out of scope (ARCHITECTURE-v1 section 13
   item 7).

## Completions are slow

**Alert:** `CompletionLatencyHigh` — p95 above 30 seconds.

1. Check GPU utilisation on the platform dashboard. Sustained 100% with high
   VRAM means queueing, not a fault.
2. `ai_chat_requests_total` by `logical_model` shows whether one route is
   responsible.
3. Confirm the model is resident: a cold load pays a one-off cost that looks
   like a latency spike.
4. `COMPUTE_TIMEOUT` bounds a single call. Raising it hides the symptom rather
   than fixing it.

## Ingestion failures

**Alert:** `IngestionFailing` — documents failing in the last 15 minutes.

A stalled corpus looks healthy from outside: uploads still return `202` and
searches still answer, from stale content.

1. The reason is on the row, not only in logs:
   ```sql
   SELECT id, title, status, error FROM documents
   WHERE status = 'failed' AND deleted_at IS NULL ORDER BY updated_at DESC;
   ```
2. Common causes: an unsupported content type (only `text/plain` and
   `text/markdown` are parsed today), content that is not valid UTF-8, or the
   embedding model being unreachable.
3. If the embedding provider was down, re-queue by setting `status = 'pending'`;
   the worker picks it up on the next poll.
4. An embedding-dimension mismatch fails at startup, not here — the schema pins
   `vector(768)` and `EMBEDDING_DIMENSIONS` is validated against it.

## Answers are ungrounded

**Alert:** `AgentAnswersUngrounded` — over half of successful runs cite nothing.

This is the failure mode retrieval exists to prevent, and it is invisible without
the metric: the answers still look fine.

1. Read a run trace: `GET /api/v1/agent/runs/{id}` shows every retrieval round,
   its score, and whether evidence was discarded as too weak.
2. If retrieval found nothing, the corpus does not cover the questions being
   asked. Check `ai_retrieval_best_score` against `AGENT_MIN_SCORE`.
3. If retrieval scored well but answers cite nothing, the model is ignoring the
   instruction to cite. That is a model capability problem, not a pipeline one —
   a small model does this inconsistently.
4. Assistants set to `retrieval: always` are instructed to say the knowledge base
   does not cover a question rather than answer from general knowledge. Consider
   moving critical assistants to `always`.

## Credential rejections

**Alert:** `CredentialRejectionSpike` — sustained rejections, possibly a probe.

1. `ai_auth_failures_total` is labelled by server-side reason. `unknown prefix`
   in volume suggests scanning; `secret mismatch` on one prefix suggests a stale
   deployment holding an old key.
2. Cross-check the audit trail:
   ```sql
   SELECT created_at, action, metadata FROM audit_logs
   WHERE outcome = 'denied' ORDER BY created_at DESC LIMIT 50;
   ```
3. Revoke a compromised key: `POST /api/v1/admin/api-keys/{id}/revoke`.
   Revocation is idempotent and keeps the original timestamp.
4. The client is only ever told "invalid api key" — the reason stays server-side
   so a probe cannot enumerate prefixes.

## Quota denials

**Alert:** `QuotaDenialsSustained` — callers throttled steadily.

1. `ai_rate_limit_denials_total` by `kind` separates request limits from tool
   limits.
2. Per-key limits live on the key record, not in configuration:
   ```sql
   SELECT name, rate_limit_per_minute, last_used_at FROM api_keys
   WHERE deleted_at IS NULL ORDER BY last_used_at DESC NULLS LAST;
   ```
3. Raise a specific key's limit rather than `DEFAULT_RATE_LIMIT_PER_MINUTE`,
   which only affects keys created afterwards.
4. The limiter **fails closed**: if Redis is unreachable every guarded request
   gets `503`. A denial spike with Redis down is a Redis incident, not a quota
   one.

---

## Backup

The AI database is the system of record for everything the platform owns:
assistants, conversations, API keys, the corpus metadata, the graph and the
audit and usage outbox. Document bytes live in `StorageProvider` and are **not**
in the database.

Three things must be backed up together, or a restore is inconsistent:

| What | Where | Command |
|---|---|---|
| AI database | `postgres-data` | Dump inside the container, then copy the file out — see below |
| Document bytes | `document-data` | `docker run --rm -v chiotron-ai_document-data:/data -v "${PWD}:/backup" alpine tar czf /backup/documents.tar.gz -C /data .` |
| Configuration | `.env` | Copy it. It holds credentials and is not in version control. |

```powershell
docker exec chiotron-ai-postgres-1 pg_dump -U chiotron_ai -Fc -f /tmp/backup.dump chiotron_ai
docker cp chiotron-ai-postgres-1:/tmp/backup.dump .\backup.dump
```

> **Do not** write `docker exec ... pg_dump -Fc chiotron_ai > backup.dump` on a
> Windows host. Windows PowerShell 5.1 applies an encoding transformation to
> redirected output, and the resulting file is a corrupt archive that
> `pg_restore` rejects with *"input file does not appear to be a valid archive"*.
> The failure appears at restore time, which is the worst possible moment to
> discover it. Writing the dump inside the container and copying the file avoids
> the pipe entirely. Verify a backup on the spot: the first five bytes of a
> valid custom-format dump are `PGDMP`.

Redis holds only rate-limit counters and is rebuildable: losing it costs one
window of quota accounting, nothing more. The Ollama model cache is also
rebuildable and is not a source of record.

Take the database dump **first**. A document present in the dump but missing
from the archive is a broken reference; a document in the archive that the
database does not know about is merely an orphaned object.

## Restore

```powershell
# 1. Stop the writers, leave the database up.
docker compose stop api portal

# 2. Copy the dump in and restore. --clean drops the existing objects first.
#    Restoring from inside the container avoids piping binary through PowerShell.
docker cp .\backup.dump chiotron-ai-postgres-1:/tmp/backup.dump
docker exec chiotron-ai-postgres-1 `
  pg_restore -U chiotron_ai -d chiotron_ai --clean --if-exists /tmp/backup.dump

# 3. Restore the documents.
docker run --rm -v chiotron-ai_document-data:/data -v "${PWD}:/backup" alpine `
  sh -c "rm -rf /data/* && tar xzf /backup/documents.tar.gz -C /data"

# 4. Start again. Migrations are idempotent and will report 0 applied.
docker compose up -d api portal
curl.exe -s localhost:8080/readyz
```

**Verified on 2026-08-18** against this platform, by destroying data rather than
merely re-importing it:

| Step | chunks | api_keys |
|---|---|---|
| Before | 3 | 7 |
| After `DELETE FROM chunks; DELETE FROM api_keys;` | 0 | 0 |
| After `pg_restore --clean --if-exists` | 3 | 7 |

The Control Plane then restarted, reported `migrationsApplied: 0`, `/readyz`
returned `200`, an API key minted before the backup still authenticated against
the restored rows, and all three documents were readable.

The first attempt at this failed: the dump had been captured with a shell
redirect and `pg_restore` rejected it as not a valid archive. That is why the
backup command above copies the file rather than piping it, and why verifying a
fresh backup is part of taking one.

**Not verified:** point-in-time recovery, and restoring into a *different*
PostgreSQL major version. Both belong to the production database's own backup
policy, which is tested separately from ERP restoration (ARCHITECTURE-v1
section 9).

## Disaster recovery

Failure domains are independent by design, so recovery is per plane:

| Lost | Effect | Recovery |
|---|---|---|
| VM5 / compute | Model calls fail with `503`. Retrieval, history and the portal keep working. `/readyz` stays `200`. | Restart the compute plane. The model cache rebuilds by re-pulling. |
| Redis | Every guarded request gets `503`: the limiter fails closed. | Restart Redis. Counters rebuild from the next window. |
| PostgreSQL | `/readyz` reports not-ready. The platform serves nothing that needs state. | Restore from the latest dump, then the document archive. |
| Document storage | Search still answers from stored chunks and embeddings; re-ingestion is impossible until the bytes return. | Restore the archive. Documents already ingested keep working. |
| Node 4 entirely | AI is unavailable. **ERP continues** — that separation is the point. | Rebuild from the images and restore both backups. |

The last row is the one to rehearse. It is also the only claim here that has not
been exercised end to end, because it needs a second host.
