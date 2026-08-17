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
| `GET /api/v1/assistants` | Assistant catalogue, filtered by the caller's company. Needs `assistants:read`. |
| `GET /api/v1/conversations` | The caller's own conversations. Needs `chat:completions`. |
| `GET/DELETE /api/v1/conversations/{id}` | One transcript, or soft-delete it. Needs `chat:completions`. |
| `POST /api/v1/documents` | Upload a document for ingestion. Needs `knowledge:write`. |
| `GET /api/v1/documents` | Documents the caller may read, plus corpus status. Needs `knowledge:read`. |
| `GET /api/v1/documents/{id}` | One document. Needs `knowledge:read`. |
| `DELETE /api/v1/documents/{id}` | Withdraw a document and its chunks. Needs `knowledge:write`. |
| `POST /api/v1/knowledge/search` | Permission-filtered hybrid retrieval. Needs `knowledge:read`. |
| `GET /api/v1/graph/neighbours` | Relationship traversal from a term. Needs `knowledge:read`. |
| `GET /api/v1/tools` | Tools the caller may actually call. Needs `tools:read`. |
| `POST /api/v1/agent/answer` | Grounded answer with citations and a run trace. Needs `agent:run`. |
| `GET /api/v1/agent/runs/{id}` | The trace for one run. Needs `agent:run`. |
| `GET/POST /api/v1/admin/api-keys` | List and mint API keys. Needs `admin:keys`. |
| `POST /api/v1/admin/assistants` | Create an assistant. Needs `admin:assistants`. |
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

Then call the API with `Authorization: Bearer <key>`. Scopes are `models:read`, `assistants:read`, `chat:completions`, `admin:keys` and `admin:assistants`; an unknown scope is rejected when the key is created, not silently ignored. Reading and deleting your own conversations falls under `chat:completions` rather than a scope of its own — a transcript is part of using chat, and it is never visible to another credential.

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

## Assistants and conversations

An assistant names the instructions and the **logical model** to use, so the portal selects an assistant and provider details stay hidden from the user (ARCHITECTURE-v1 section 10). Migration `0003` seeds a `general` assistant on the default route; more are created through `POST /api/v1/admin/assistants`. The catalogue is filtered by the caller's company in SQL, and instructions are never returned by it — they are policy the operator wrote, not something every caller needs to read back.

`POST /api/v1/chat/completions` accepts either form, never both:

| Form | Body | Behaviour |
|---|---|---|
| Stateless | `messages` | The caller holds the transcript. Nothing is stored. |
| Stored | `message` plus `assistant` or `conversationId` | The platform holds the transcript, prepends the assistant's instructions and replays recent turns. |

A conversation is bound to the assistant it started with, so a later request cannot quietly switch its behaviour. Every read is scoped to the credential that created it: another key's id reads as `404`, not `403`, so an id cannot be probed for existence. Deletion is soft, keeping audit references intact.

`PERSIST_PROMPTS=false` turns off prompt logging: turns are still recorded so history keeps its titles and counters, but message text is not stored and redacted turns are skipped when replaying context. `HISTORY_TURN_LIMIT` caps how much of a transcript is sent to the model — a transcript grows without limit, a context window does not.

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

## Knowledge platform

A document is uploaded, stored through `StorageProvider`, then picked up by the ingestion worker: parse, chunk, embed, store. Upload returns `202` with the document in `pending`; the worker moves it to `ready` or to `failed` with the reason on the row, so an operator can fix the input without reading logs.

**Source ACL metadata travels with every chunk** (ARCHITECTURE-v1 section 6). Company, department and classification are copied onto each chunk, and retrieval applies all three as SQL predicates *before* ranking — the predicates are repeated inside each candidate query rather than shared through a CTE, so the planner can still reach the HNSW and GIN indexes.

The three ABAC dimensions come from the credential, never from the request: an API key carries a company, a department and the highest classification it may read. A caller also cannot file a document above its own clearance, which would create content it could not itself retrieve.

```powershell
docker compose run --rm --no-deps api apikey create -name analyst `
  -classification internal -department finance -company acme
```

`CLASSIFICATION_LEVELS` is the ladder, least sensitive first — it is configuration because the policy of record is an open decision (section 13 item 5). An unknown level is never readable: `Allows` fails closed, including for a zero-valued policy.

**Retrieval is hybrid**: pgvector cosine similarity fused with PostgreSQL full-text rank by reciprocal rank fusion. Two details matter for it to work at all:

- The text search configuration is `simple`, not `english`. The corpus is multilingual and English stemming would mangle Thai, Chinese, Japanese and Burmese content.
- `plainto_tsquery` ANDs every term, and `simple` strips no stopwords, so a question like *"how much VRAM does the development GPU have"* matched nothing — the keyword half of the hybrid was dead. The operators are rewritten to OR, which is what a retrieval query means.

Embeddings come from `nomic-embed-text` at 768 dimensions, which the `chunks` table pins as `vector(768)`. Changing the embedding model changes that width, so it needs a migration and a re-embedding pass rather than a configuration flip — `EMBEDDING_DIMENSIONS` is validated against the schema at startup to make that explicit.

Only `text/plain` and `text/markdown` are accepted today. PDF and office formats need a parsing dependency and arrive with the connector work; rejecting them is better than storing bytes nothing can read.

## Relationship graph

Ingestion projects each document into `graph_nodes`, `graph_edges` and `graph_mentions` — AI-owned tables, so GraphRAG needs no infrastructure decision before there is a graph worth running on (ARCHITECTURE-v1 section 6). Everything above `internal/graph` depends on the `Provider` interface, which is what makes Neo4j an adapter change rather than an orchestration change.

**Extraction is deterministic, not model-driven.** The same document always produces the same graph, which is what makes an edge weight meaningful and a traversal reproducible. It finds title-case names and short all-caps identifiers (`VM4`, `S3`), joins adjacent words into one name, and allows an acronym *inside* a name — `Enterprise AI Platform` is one entity — but never at the end, so `Control Plane VM4` stays two things. Casing variants collapse: `CONTROL PLANE` and `Control Plane` are one node.

Edges record `mentioned_in` (entity → document) and `co_occurs_with` (entity ↔ entity in the same chunk). The second is deliberately untyped: naming a relationship needs a capable model, and inventing a label would be worse than admitting the edge only says the two appear together. Co-occurrence is per chunk, not per document — two entities in one passage are plausibly related, two in a fifty-page document are not.

**Access is filtered at every hop, not just the seed.** A readable node must not become a bridge to an unreadable one. Verified with two documents sharing one entity:

| Clearance | Reached from `Control Plane` |
|---|---|
| `restricted` | Control Plane, Gateway Service, **Project Falcon**, both documents |
| `internal` | Control Plane, Gateway Service, Gateway architecture only |

A node several documents mention keeps the **least** sensitive of their classifications: knowing an entity exists is only as sensitive as the least restricted document that says so, and taking the first writer's level would hide an entity from readers whose own documents mention it. Edges keep their own document's level, which is what actually stops the walk.

Edges are stored per document, so re-ingesting replaces a contribution instead of doubling every weight, and withdrawing a document removes exactly what it added — including any node left with nothing evidencing it. `mention_count` is derived from the mention rows rather than maintained as a counter, so it cannot drift from its evidence.

Both `GRAPH_DEPTH` and `GRAPH_MAX_NODES` exist because either alone is insufficient: depth without a node cap can still fan out to the whole graph. The configured depth is a ceiling a caller may lower but not raise.

## Agentic retrieval

`POST /api/v1/agent/answer` plans, retrieves, optionally calls tools, then synthesises an answer with citations. Every run is recorded step by step, so an answer can be explained afterwards: which searches ran, what they scored, which tools were called and which passages the answer was built from. `GET /api/v1/agent/runs/{id}` returns that trace.

**The planner is deterministic policy, not a model choosing its own next move.** A rule can be read, tested and audited; asking a 0.5B model to plan its own retrieval would make every answer's shape unexplainable. The rules are:

- Retrieval is assistant policy — `off`, `auto` or `always`. In `auto` a question of fewer than three words is not treated as a corpus query.
- A round that scores below `AGENT_MIN_SCORE` triggers one follow-up query anchored on the strongest document's title, which pulls in its neighbouring chunks. With nothing to anchor on, the agent stops rather than repeating the same search.
- Evidence still below the threshold is **discarded**, not passed along weakly. An assistant set to `always` is then told to say the knowledge base does not cover the question instead of answering from the model alone.
- `AGENT_MAX_STEPS` counts investigation only. The plan record and the synthesis do not consume it, and a skipped step costs nothing.

**Grounding is measured, not assumed.** The response reports `citations`, `citedIndices` and `grounded`: a model can be handed passages and ignore them, and the platform says so rather than implying the answer came from the corpus.

**Conflict is flagged, not resolved.** When the two strongest passages come from different documents of comparable *semantic* relevance, the run is marked `conflicted` and the prompt asks for both positions. The comparison uses cosine similarity rather than the fused score on purpose: RRF scores compress by construction — the top results differ by fractions of 1/61 — so a relative margin on them fires almost every time and says nothing about whether the documents are on the same subject.

**Tools are a controlled registry.** Each declares the scope a caller must hold, is rate limited per key *and* per tool through the same Redis counter as HTTP requests, and writes a `tool_calls` row on every path including denied and throttled. `knowledge.search` derives the caller's clearance inside the tool: an agent must not be able to widen the access of the person it acts for. The shipped tools are `knowledge.search`, `compute.health` and `platform.time`.

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

**Languages.** Every string the platform owns goes through `t(key)` in Thai, English, Chinese, Burmese and Japanese, with `formatDate`/`formatNumber` following the active language (ARCHITECTURE-v1 section 10). The choice is stored per browser and defaults to the closest match from `navigator.languages`; `<html lang>` follows it so screen readers and hyphenation behave.

The catalogue is typed from the English keys, so **a missing translation in any language fails the build** rather than silently falling back — verified by deleting one Burmese string and watching `tsc` reject it:

```
src/i18n.ts(636,7): error TS2741: Property '"chat.error.catalogue.title"' is missing
  in type '{ ... }' but required in type 'Catalogue'.
```

Two bodies of text are deliberately **not** translated and are labelled as such in the UI: the twelve governance rules with the ten stack entries, and the roadmap's phase and milestone names. They mirror `docs/ARCHITECTURE-v1.md` and should be translated together with it, by a reviewer who can check the terminology. The Chinese, Japanese and Burmese UI strings are a first pass and want a native review before production.

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
