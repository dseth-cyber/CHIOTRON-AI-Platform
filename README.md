# CHIOTRON Enterprise AI Platform

Development foundation for Node 4. The Docker composition mirrors the intended separation:

- `portal`: unified User Portal entry point on `http://localhost:5173`
- `api`: VM4-style Control Plane / AI Gateway entry point on `http://localhost:8080`
- `postgres`: platform-owned PostgreSQL with pgvector
- `redis`: cache and future queue coordination
- `ollama`: VM5-style local Compute Plane, intentionally isolated from the host and enabled only with the `compute` profile

## Run locally

```powershell
Copy-Item .env.example .env
docker compose up --build -d
```

Start the local Compute Plane when wanted:

```powershell
docker compose --profile compute up -d ollama
```

Ollama and model credentials are deliberately never exposed to browsers.

## Control Plane endpoints

| Endpoint | Purpose |
|---|---|
| `GET /healthz` | Liveness. Answers without touching dependencies, so a database outage does not trigger a pointless restart. |
| `GET /readyz` | Readiness. Probes PostgreSQL and Redis and returns `503` with per-dependency detail when one is unavailable. |
| `GET /metrics` | Prometheus scrape endpoint. |
| `GET /api/v1/platform` | Platform discovery. |
| `GET /api/v1/me` | The calling credential's own name, scopes, company and quota. Any authenticated caller. |
| `GET /api/v1/compute/health` | Per-provider compute-plane status and loaded models. Needs `models:read`. |
| `GET /api/v1/models` | Logical models, the route behind each one, and whether the upstream model is loaded. Needs `models:read`. |
| `POST /api/v1/chat/completions` | Non-streaming completion. Needs `chat:completions`. |
| `GET/POST /api/v1/admin/api-keys` | List and mint API keys. Needs `admin:keys`. |
| `POST /api/v1/admin/api-keys/{id}/revoke` | Revoke a key. Needs `admin:keys`. |
| `GET /api/v1/admin/outbox` | Unpublished usage and audit backlog. Needs `admin:keys`. |

Liveness, readiness, metrics and platform discovery are unauthenticated: probes and scrapes carry no credential. Everything else requires a key.

## Authentication

API keys are platform-owned credentials — hashed, scoped, rate-limited, expirable and auditable, with the raw value shown once only (ARCHITECTURE-v1 section 5). They are separate from the JWTs the existing Identity Service issues, which arrive with the identity integration.

A key looks like `ceap_<hex prefix>_<base64url secret>`. Only the prefix and a SHA-256 of the secret are stored. The secret is 256 bits of randomness rather than a user-chosen password, so a password-hardening KDF would add per-request latency without adding meaningful resistance.

Minting the first key over HTTP is impossible — no key holds `admin:keys` yet — so it is done through the binary rather than through a master credential in the environment:

```powershell
docker compose run --rm --no-deps api apikey create -name bootstrap -scopes models:read,chat:completions,admin:keys
```

Then call the API with `Authorization: Bearer <key>`. Scopes are `models:read`, `chat:completions` and `admin:keys`; an unknown scope is rejected when the key is created, not silently ignored.

`X-Active-Company` is honoured only after the credential is validated and only when it matches what the key is entitled to; anything else is a `403`.

### Rate limits

Each key carries its own requests-per-minute quota (`DEFAULT_RATE_LIMIT_PER_MINUTE` when the creating call does not name one). Counters live in Redis under `ai:rate-limit:`. Every response carries `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset`; a throttled one adds `Retry-After`.

The limiter **fails closed**: if Redis is unreachable the request gets a `503` rather than bypassing the quota. Redis is already a declared readiness dependency, so an outage should shed load rather than silently remove the ceiling that protects the compute plane.

### Usage and audit outbox

Every model call writes a `usage_events` row and every denied or administrative action writes an `audit_logs` row, both with `published_at` NULL. These drain to `ai.usage.v1` and `ai.audit.v1` once Kafka is deployed (ARCHITECTURE-v1 section 7); until then the tables are the durable record and nothing is lost by the broker being absent. `GET /api/v1/admin/outbox` reports the backlog.

Writing an audit row never fails the request that produced it — losing the action is worse than losing its audit line, and the failure is still logged at error level.

## Compute plane

Business code depends on the `provider.LLM` interface, never on a vendor SDK, so replacing Ollama with vLLM, NIM or an external API is an adapter plus configuration change (ARCHITECTURE-v1 section 4). Only the Ollama adapter exists today; any other `COMPUTE_PROVIDER` fails at startup with a clear message rather than silently doing nothing.

Callers name a **logical model**; `MODEL_ROUTES` decides which provider and upstream model serves it:

```
MODEL_ROUTES=default=ollama/qwen2.5:0.5b,fast=ollama/qwen2.5:0.5b
DEFAULT_MODEL=default
```

The provider and model are split on the first slash, so upstream names may contain colons. A route naming an unregistered provider, a duplicate logical id, or a `DEFAULT_MODEL` with no route stops the process at startup instead of failing on a user's first request.

**Failure isolation is deliberate.** The compute plane is not part of `/readyz`: losing VM5 degrades model calls only and the Control Plane stays ready (ARCHITECTURE-v1 section 9). With Ollama unreachable, `/readyz` still returns `200`, `/api/v1/compute/health` reports `unavailable` with the underlying reason, `/api/v1/models` marks routes unavailable, and a chat call returns `503` — never a Control Plane `500`.

### Streaming

Send `"stream": true` and the same endpoint answers with Server-Sent Events instead of a single JSON body:

```
data: {"content":"1"}
data: {"content":"\n"}
data: {"done":true,"finishReason":"stop","usage":{"promptTokens":42,"completionTokens":10,"totalTokens":52}}
data: [DONE]
```

Token counts are only known when generation finishes, so they ride on the final frame. Usage is recorded identically in both modes.

Response headers are written on the first chunk rather than up front. Until something has actually been produced a failure can still be reported with a real HTTP status — an unreachable compute plane is a `503`, not an error buried inside a `200` stream. Once the stream is established, a mid-flight failure arrives as an `event: error` frame.

`StreamingLLM` is an optional part of the provider contract. A provider that cannot stream still serves a `stream: true` request: the whole response is delivered as one chunk, so callers never have to know which backend answered.

## Configuration and schema

All runtime settings come from the environment; see `.env.example` for the full list with defaults. Invalid configuration fails at startup with every problem reported at once, and connection strings never reach the logs.

The Control Plane owns its schema. Migrations live in `control-plane/internal/migrations`, are embedded in the binary and are applied at startup under a PostgreSQL advisory lock, so a schema change reaches an existing database instead of requiring a volume wipe. `infra/postgres/init.sql` is reduced to privileged bootstrap (`CREATE EXTENSION vector`) because it only ever runs on an empty data volume.

To add a migration, create `control-plane/internal/migrations/NNNN_description.sql` and rebuild. Applied migrations are recorded in `schema_migrations` with a checksum; editing an already-applied file is rejected at startup rather than silently ignored.

## Observability

The platform reuses the existing monitoring stack rather than running one of its own.

- **Metrics** are always available at `/metrics`: Go runtime, process and OpenTelemetry HTTP server metrics, plus a `target_info` series carrying `service.name`, `service.version` and `deployment.environment.name`. No collector needs to be reachable for this to work.
- **Traces** are exported over OTLP/HTTP only when `OTEL_EXPORTER_OTLP_ENDPOINT` is set, so local development runs unchanged without a collector. `OTEL_TRACES_SAMPLER_ARG` sets the sample ratio.
- **Log correlation**: request logs carry `traceId` and `spanId` whenever a span is active, which is what joins Loki logs to Tempo traces.
- `/healthz`, `/readyz` and `/metrics` are excluded from tracing and from HTTP server metrics — probes and scrapes run continuously and would otherwise dominate both.

Build metadata comes from the `VERSION` build argument and is reported by `/healthz` and `/api/v1/platform`:

```powershell
docker compose build --build-arg VERSION=0.1.0 api
```

## Checks

`.github/workflows/ci.yml` runs module tidiness, `gofmt`, `go vet`, `go test -race`, the portal build, `docker compose config` and both image builds.

The same checks run locally through Docker, since this development host has no Go or Node toolchain installed:

```powershell
powershell -File scripts/check.ps1
```

## Portal

The portal is the only browser client and it talks to the Control Plane only (ARCHITECTURE-v1 section 2). It reads `GET /api/v1/platform` without a credential, so the shell renders before anyone connects; models, compute status and the chat workspace need an API key.

**Connecting.** The topbar's *API key* dialog stores a key in `sessionStorage`, so it dies with the tab and is never written into the image. `GET /api/v1/me` then tells the portal what that key may do, which is what drives navigation — the Chat item is disabled without `chat:completions`. That filtering is convenience only: the backend authorizes every request regardless of what the UI chose to show.

This is a development bridge. Once the Identity Service issues JWTs to the portal, the browser stops holding a platform credential; until then, connect the narrowest key that does the job and keep `admin:keys` out of the browser.

**Chat.** The workspace streams over SSE, picks its model list from `/api/v1/models` (defaulting to whatever the gateway calls default, never a hard-coded model name), and shows real token counts and latency per turn. `EventSource` cannot send an `Authorization` header, so the stream is read with `fetch` and reassembled across network chunks.

**Live figures.** The Developer Portal stat cards read environment, version, capabilities, model availability and compute status from the API. Only the roadmap remains local state — it is a planning artefact, not platform data.

Type checking is part of the build: `npm run build` runs `tsc --noEmit` first, so a type error fails the image. Dependencies are pinned by `package-lock.json` and installed with `npm ci`.

The next development phase should add identity/JWT middleware, then assistant and conversation APIs on top of the compute registry.

## After changing source

Both images build their artefacts at image build time — there is no bind mount or dev server, so edits are invisible until the image is rebuilt:

```powershell
docker compose up -d --build portal   # after editing web/
docker compose up -d --build api      # after editing control-plane/
```

## Architecture boundary

`portal -> AI Gateway / Orchestrator -> authorized tools and compute providers`

ERP and users must never address the Compute Plane directly. Production placement puts `api`, portal, PostgreSQL, and Redis in VM4, while Ollama/vLLM runs on GPU-passthrough VM5 over a private network.
