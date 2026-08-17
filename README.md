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
| `GET /api/v1/compute/health` | Per-provider compute-plane status and loaded models. |
| `GET /api/v1/models` | Logical models, the route behind each one, and whether the upstream model is loaded. |
| `POST /api/v1/chat/completions` | Development only, see below. |

## Compute plane

Business code depends on the `provider.LLM` interface, never on a vendor SDK, so replacing Ollama with vLLM, NIM or an external API is an adapter plus configuration change (ARCHITECTURE-v1 section 4). Only the Ollama adapter exists today; any other `COMPUTE_PROVIDER` fails at startup with a clear message rather than silently doing nothing.

Callers name a **logical model**; `MODEL_ROUTES` decides which provider and upstream model serves it:

```
MODEL_ROUTES=default=ollama/qwen2.5:0.5b,fast=ollama/qwen2.5:0.5b
DEFAULT_MODEL=default
```

The provider and model are split on the first slash, so upstream names may contain colons. A route naming an unregistered provider, a duplicate logical id, or a `DEFAULT_MODEL` with no route stops the process at startup instead of failing on a user's first request.

**Failure isolation is deliberate.** The compute plane is not part of `/readyz`: losing VM5 degrades model calls only and the Control Plane stays ready (ARCHITECTURE-v1 section 9). With Ollama unreachable, `/readyz` still returns `200`, `/api/v1/compute/health` reports `unavailable` with the underlying reason, `/api/v1/models` marks routes unavailable, and a chat call returns `503` — never a Control Plane `500`.

`POST /api/v1/chat/completions` exists only to exercise the adapter before the Gateway's JWT middleware lands. It is **absent unless `DEV_UNAUTHENTICATED_CHAT=true`**, and logs a warning at startup when enabled. Streaming is deliberately not implemented here: SSE ships with the Gateway, together with authentication and quota.

```powershell
$env:DEV_UNAUTHENTICATED_CHAT = "true"; docker compose up -d api
```

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

Known gaps: the portal has no `package-lock.json` (so CI resolves current versions rather than pinned ones) and no `tsconfig.json` or `vite.config.ts`, which means `npm run build` transpiles TypeScript without type checking.

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
