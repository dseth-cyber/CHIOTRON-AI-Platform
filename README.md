# CHIOTRON Enterprise AI Platform

> **Modern Enterprise AI Platform with Single Go Binary Architecture (Go 100%)**  
> ปัญญาประดิษฐ์ระดับองค์กร: รวม AI Gateway, Multi-Tier Model Router, Vector RAG (pgvector), GraphRAG, Agent ReAct Engine, Security RBAC และ Web Portal ไว้ใน Go Binary เดียว

---

## 🚀 คู่มือการติดตั้งและเริ่มต้นใช้งาน (Quick Start Guide)

### 📋 ข้อกำหนดของระบบ (Prerequisites)
- **Docker Desktop** (พร้อมเปิดใช้งาน WSL2 บน Windows หรือ Docker Engine บน Linux)
- **NVIDIA GPU Driver & Container Toolkit** (กรณีต้องการรัน Local AI Model บนการ์ดจอ)
- **RAM:** ขั้นต่ำ 8GB (แนะนำ 16GB+ หากรัน LLM ในเครื่อง)

---

### ⚡ 3 ขั้นตอนติดตั้งและรันระบบ (One-Command Setup)

#### 1. คัดลอกไฟล์การตั้งค่า Environment (.env)
```powershell
Copy-Item .env.example .env
```
*(หรือบน Linux/macOS: `cp .env.example .env`)*

#### 2. สั่งรันระบบหลัก (Single Go Binary Platform)
```powershell
docker compose up -d --build
```
ระบบจะทำการ Build และ Start เซอร์วิสทั้งหมดขึ้นมาโดยอัตโนมัติ:
* **`api` (Single Go Binary):** ให้บริการทั้ง Web Portal และ AI Gateway บนพอร์ต `http://localhost:5173` และ `http://localhost:8080`
* **`postgres` (PostgreSQL 16 + pgvector):** จัดการฐานข้อมูลและ Vector Embeddings
* **`redis` (Redis 7):** จัดการ Semantic Cache และ Rate-Limit

#### 3. เปิดใช้งาน Local Compute Plane (Ollama GPU Engine)
```powershell
docker compose --profile compute up -d ollama
```

---

### 🔑 การสร้าง Admin API Key และดาวน์โหลดโมเดล

#### 1. สร้าง Admin API Key ครั้งแรก (Bootstrap Key)
สร้างคีย์ผู้ดูแลระบบผ่าน Go CLI โดยตรง:
```powershell
docker compose exec api /control-plane apikey create --name "Admin Key" --admin
```
*(ระบบจะพิมพ์คีย์รูปแบบ `ceap_...` ออกมา ให้คัดลอกเก็บไว้สำหรับกรอกในหน้าเว็บ)*

#### 2. ดาวน์โหลดโมเดล AI ภาษาไทย/อังกฤษ และ Embedding
```powershell
# ดาวน์โหลดโมเดล LLM ขนาดเล็กสำหรับทดสอบ (กิน VRAM ~1GB)
docker compose exec ollama ollama pull qwen2.5:0.5b

# ดาวน์โหลดโมเดล Embedding สำหรับค้นหาเอกสาร (RAG)
docker compose exec ollama ollama pull nomic-embed-text
```

#### 3. เข้าใช้งานระบบผ่านเว็บเบราว์เซอร์
* 🌐 **User Portal & AI Workspace:** [http://localhost:5173](http://localhost:5173) หรือ [http://localhost:8080](http://localhost:8080)
* 📊 **ระบบตรวจสอบสถานะ (Health Check):** [http://localhost:8080/healthz](http://localhost:8080/healthz)
* 📈 **ดัชนีชี้วัด (Prometheus Metrics):** [http://localhost:8080/metrics](http://localhost:8080/metrics)

---

## 🏗️ สถาปัตยกรรมระบบ (System Architecture)

```
[ Browser / Client ]
        │
        ▼ (Port 5173 / Port 8080)
┌────────────────────────────────────────────────────────┐
│  CHIOTRON Enterprise AI Platform (Single Go Binary)    │
│  - Web Portal UI (Embedded via //go:embed)             │
│  - AI Gateway & Model Router (Go 100%)                 │
│  - Security, RBAC & Token Limiting (Go 100%)           │
│  - Vector RAG & Knowledge Platform (Go 100%)           │
│  - Agent Planner & Tool Registry (Go 100%)             │
└───────────────────────┬────────────────────────────────┘
                        │
       ┌────────────────┴───────────────┐
       ▼                                ▼
[ PostgreSQL 16 + pgvector ]      [ Local Ollama GPU Engine ]
```

---

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
| `GET /api/v1/conversations` | The caller's own conversations. `?trash=true` reads deleted ones. Needs `chat:completions`. |
| `GET/DELETE /api/v1/conversations/{id}` | One transcript, or soft-delete it. Needs `chat:completions`. |
| `POST /api/v1/conversations/{id}/restore` | Undo a soft delete. Needs `chat:completions`. |
| `POST /api/v1/documents` | Upload a document for ingestion. Needs `knowledge:write`. |
| `GET /api/v1/documents` | Documents the caller may read, plus corpus status. `?trash=true` reads withdrawn ones. Needs `knowledge:read`. |
| `GET /api/v1/documents/{id}` | One document. Needs `knowledge:read`. |
| `DELETE /api/v1/documents/{id}` | Withdraw a document and its chunks. Reversible; the bytes are kept. Needs `knowledge:write`. |
| `POST /api/v1/documents/{id}/restore` | Return a withdrawn document and requeue ingestion. Needs `knowledge:write`. |
| `DELETE /api/v1/documents/{id}/purge` | Destroy a withdrawn document permanently. Requires `?confirm=` to repeat the id. Needs `knowledge:write`. |
| `POST /api/v1/knowledge/search` | Permission-filtered hybrid retrieval. Needs `knowledge:read`. |
| `GET /api/v1/graph/neighbours` | Relationship traversal from a term. Needs `knowledge:read`. |
| `GET /api/v1/favorites` | The caller's own marks, resolved against what it may still read. Needs `chat:completions`. |
| `PUT/DELETE /api/v1/favorites` | Mark or unmark an assistant, conversation or document. Needs `chat:completions`. |
| `GET /api/v1/tools` | Tools the caller may actually call. Needs `tools:read`. |
| `POST /api/v1/agent/answer` | Grounded answer with citations and a run trace. Needs `agent:run`. |
| `GET /api/v1/agent/runs/{id}` | The trace for one run. Needs `agent:run`. |
| `GET /api/v1/admin/providers` | Providers, routes, adapter kinds and classification levels. Needs `admin:keys`. |
| `POST/PATCH/DELETE /api/v1/admin/providers[/{slug}]` | Add, edit or remove a model backend. Needs `admin:keys`. |
| `POST /api/v1/admin/providers/{slug}/check` | Test a provider's endpoint and credential. Needs `admin:keys`. |
| `PUT /api/v1/admin/routes`, `DELETE /api/v1/admin/routes/{logical}` | Point a logical model at a provider. Needs `admin:keys`. |
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

## Model providers and routing

Which backend answers is a row in the database, not an environment variable (ARCHITECTURE-v1 sections 46 and 53). Providers and routes are managed from the portal's **Providers** page or the admin endpoints above, and a change applies to the running process — the registry pointer is swapped under an atomic, so in-flight requests finish against the table they started with and nothing needs a restart.

Two adapters cover most of the market:

| Kind | Backends |
|---|---|
| `ollama` | The local compute plane |
| `openai-compatible` | OpenAI, Azure OpenAI, Groq, Together, OpenRouter, DeepSeek, Mistral, **vLLM, NVIDIA NIM**, LM Studio |
| `anthropic` | The Messages API, which is not OpenAI-compatible and needs its own adapter |

This is what makes development without a GPU possible: point `default` at `openai/gpt-4o-mini` from the Providers page, exercise chat, streaming, RAG and the agent end to end, then point it back at Ollama. No code changes, no redeploy.

**Credentials are sealed with AES-GCM** under `CONFIG_ENCRYPTION_KEY`, which stays an environment variable because a key stored in the database it protects would protect nothing. Generate one with `openssl rand -base64 32`. Without it, providers that need no credential still work and providers that need one **cannot be created** — storing a key in the clear is deliberately not offered. No endpoint ever returns a credential; the UI gets `hasCredential` and the last four characters.

### The egress ceiling

Permission-aware retrieval answers *what may this caller read*. Pointing a route at a hosted API raises a different question — *what may this provider be told* — and nothing in the platform asked it until this work. Without a ceiling, adding a cloud provider would silently turn RAG into a data export.

Every provider carries a `max_classification`. After the agent assembles its context and **before any bytes are sent**, the run is refused if any retrieved passage sits above that ceiling. The comparison is against what was actually retrieved rather than the caller's clearance, so a confidential-cleared user whose question happens to draw only on public passages is still answered by a public-only provider.

New providers default to the **least sensitive** level. A loose default is the one nobody comes back to tighten.

Verified against the running platform: with `openai` at `public`, an agent question drawing on `internal` documents was refused with `403` and the run trace recorded a `denied` step; nothing reached the network. Raising the ceiling to `internal` let the same question through, and the request genuinely arrived at `api.openai.com`, which rejected the deliberately fake key with a `401`. The ceiling was the only thing standing between the corpus and the internet.

The ceiling governs what the platform has classified. It **cannot** govern what a user types into a chat box, and the Providers page says so: on a route pointing at an external provider, anything typed leaves the building.

## Compute plane

Business code depends on the `provider.LLM` interface, never on a vendor SDK, so replacing Ollama with vLLM, NIM or an external API is an adapter plus configuration change (ARCHITECTURE-v1 section 4). Only the Ollama adapter exists today; any other `COMPUTE_PROVIDER` fails at startup with a clear message rather than silently doing nothing.

Callers name a **logical model** and the registry decides which provider and upstream model serves it. `COMPUTE_PROVIDER`, `OLLAMA_BASE_URL`, `MODEL_ROUTES` and `DEFAULT_MODEL` seed that table on first start against an empty database, so an existing deployment keeps working without re-entering what its environment already held. **After the seed, the database is the source of truth and those variables are ignored** — two places to change one thing is one too many.

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

### MCP servers

Model Context Protocol servers are reached over the network by a governed client, never spawned as child processes: the Control Plane image ships no runtime to spawn them with, and a tool running inside the gateway shares its blast radius. Servers are separate deployables the platform is a client of.

A remote tool joins the **same registry** as a built-in — same scope check, same per-tool rate limit, same `tool_calls` audit row. There is one governance path, not two, because the weaker of two paths becomes the way in.

Register a server in `mcp_servers`. Three fields carry the governance:

- `required_scope` belongs to the **server**, not to the remote tool. A server decides what tools it offers and may change them, so it cannot choose its own permissions.
- `allowed_tools` is an allowlist. Empty accepts everything advertised, which suits development and is too permissive for production — a server could otherwise widen its own surface after approval simply by advertising more.
- `headers` holds credentials for reaching the server and is **never returned by any API**. A caller authorized to use a tool is not authorized to learn how the platform authenticates to it.

Arguments are validated against the tool's declared JSON Schema *before* the call leaves the platform. A malformed call still crosses a trust boundary and still costs a rate-limit slot, and an undeclared property is more likely a caller typo than a feature. Keywords the validator does not implement are passed through rather than refused — rejecting a legitimate call over an unsupported keyword is worse than the server checking it too.

Discovery runs at startup and never blocks it. A server that cannot be reached is skipped with the reason written to its row, so an operator sees why its tools are missing without reading logs — the same rule the compute plane follows.

Verified against the official `@modelcontextprotocol/server-everything` over Streamable HTTP: 13 tools advertised, the allowlist admitted 2, the agent called one, a key without `tools:read` was refused, a malformed call was denied before leaving the platform, and stopping the server left the platform healthy with the failure recorded.

**Tools are a controlled registry.** Each declares the scope a caller must hold, is rate limited per key *and* per tool through the same Redis counter as HTTP requests, and writes a `tool_calls` row on every path including denied and throttled. `knowledge.search` derives the caller's clearance inside the tool: an agent must not be able to widen the access of the person it acts for. The shipped tools are `knowledge.search`, `compute.health` and `platform.time`.

## Observability

The platform reuses the existing monitoring stack rather than running one of its own. Dashboards and alert rules are files in `infra/`, so they are reviewed and versioned here and loaded by the production Prometheus and Grafana. A development-only `observability` profile runs its own pair so those artefacts can be validated against a running platform before handover:

```powershell
docker compose --profile observability up -d   # Prometheus :9090, Grafana :3000
docker compose --profile compute up -d gpu-exporter
```

### Metric contract

The names are the contract — dashboards and alert rules are written against them, so they change only with the dashboards that read them.

| Metric | What it answers |
|---|---|
| `ai_chat_requests_total`, `ai_chat_tokens_total`, `ai_chat_latency_seconds` | Volume, token consumption and latency per logical model |
| `ai_chat_cost_total` | Configured cost of consumed tokens |
| `ai_agent_runs_total`, `ai_agent_steps_total` | Agent outcomes, including whether answers were grounded and conflicted |
| `ai_retrieval_best_score` | Whether retrieval is actually finding anything |
| `ai_tool_calls_total`, `ai_tool_latency_seconds` | Tool use, including denied and throttled calls |
| `ai_ingestion_documents_total`, `ai_ingestion_chunks_total` | Whether the corpus is still growing |
| `ai_auth_failures_total`, `ai_rate_limit_denials_total` | Rejected credentials and quota refusals |
| `gpu_memory_*`, `gpu_utilisation_ratio`, `gpu_temperature_celsius` | Compute-plane capacity, from the exporter on VM5 |

No label carries an API key or a company. Metrics are scraped and retained by a shared Prometheus, and per-tenant labels would both explode cardinality and put tenancy in a store with weaker access controls than the audit tables that already hold it.

**Cost is a configured rate, not an invoice.** Local inference bills nothing, so `TOKEN_PRICES` declares what a thousand tokens of a logical model is worth. A model with no declared rate reports no cost, which is more honest than inventing one.

GPU metrics come from `cmd/gpu-exporter`, deployed on the compute plane because only a process with the NVIDIA runtime can see the card. It shells out to `nvidia-smi` on a timer rather than on scrape, so a hung driver call cannot stall Prometheus, and it keeps the previous readings on failure so a transient error shows as a stale scrape rather than a GPU that appears to have vanished.

Alert rules live in `infra/prometheus/alerts.yml` and every one points at a section of [docs/RUNBOOKS.md](docs/RUNBOOKS.md) — an alert nobody knows how to act on is noise. The runbooks also cover backup, restore and per-plane disaster recovery; the restore procedure has been exercised against this platform and records what was and was not verified.

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

The portal is the only browser client and it talks to the Control Plane only (ARCHITECTURE-v1 section 2). It reads `GET /api/v1/platform` without a credential, so the shell renders before anyone connects; models, compute status and everything in the workspace need an API key.

**Pages.** Navigation is grouped the way section 10 lists them:

| Group | Pages |
|---|---|
| Workspace | Home, New chat, Analyze, Create, History, Assistants, Favorites, Shared chats |
| Knowledge | Documents, Search |
| Platform | Developer Portal, Roadmap, Settings |

Section 11 also names five workspaces. Chat, Analyze, Create and Search exist. The ERP workspace does not, and cannot until the ERP API inventory arrives — it is Phase 8 and Phase 10 work, not a page that can be written now.

Eight of the nine pages section 10 names are built. **Shared chats is not, and the page in that slot says so** rather than pretending: a conversation belongs to an API key, not to a person, so two people sharing a key already share a history and one person holding two keys shares none. Sharing needs an owner that survives a key rotation, which is the Identity Service contract section 13 item 2 is waiting on.

**Shared components.** `SearchableSelect`, `DataTable`, `EmptyState` and `ConfirmDialog` under `web/src/components/` are used by every page, so sorting, column selection, paging, loading and empty states behave identically wherever they appear. `web/src/theme.ts` holds the one theme: it is installed as CSS custom properties before the first render, and a component that hard-codes a colour is how a design drifts one page at a time.

There is no native `<select>` and no `confirm()` anywhere in `web/src` (sections 36 and 43). Tables offer 10/20/50/100 rows a page and report the range rather than only the total, because "20 rows" tells somebody on page three nothing about what they are looking at.

**Analyze.** Reads a `.txt`, `.md`, `.csv`, `.tsv`, `.json` or `.log` file **in the browser** and runs summarize, key points, translate, rewrite, extract-a-table or a free question over it. The file is never uploaded: analysing something is not the same as filing it in the corpus, and a document somebody only wanted a summary of should not acquire a classification, an owner and a permanent row. CSV and TSV are parsed for a preview table, quoted fields included, because a preview that splits on a quoted comma shows a table that does not match the file. Long files are truncated at 40,000 characters and the page says so. Excel, PDF and image analysis are **not** offered: parsing the first two needs a dependency the platform does not carry, and images need a vision model the compute plane does not run.

**Create.** Drafts a report, document, email, presentation outline or code from a brief, with audience, tone, length and output language, then streams it with copy and download. Image and chart generation are **not** offered: the compute plane runs one 0.5B text model on a 2 GB card and there is no image model to route to.

Both are stateless — they send `messages` rather than an assistant and a conversation — so a file analysis or a draft never turns up in somebody's history.

**Trash and restore.** Nothing is destroyed by one click (section 43). Deleting a conversation or a document is a soft delete behind a `ConfirmDialog`; both pages have a trash toggle that lists what was thrown away under the same ACL predicates, and restore puts it back. Restoring a document returns 202 rather than 200 and sets it back to `pending`, because the row is back but the chunks are not until the worker re-ingests. Permanent destruction is a separate verb on a separate path, reachable only for something already in the trash, and refuses unless the request repeats the document id — the UI asks for the title to be typed back before it sends that.

Verified against the running stack: withdrawing a document dropped it from the live listing into the trash, left the stored object on disk, and took the graph from 7 nodes to 0; restoring it returned 202, the worker re-ingested it to `ready` with its chunk back, and the graph returned to 7 nodes and 21 edges. Purging refused on a live document, refused with no confirmation and with a mismatched one, then succeeded and removed the stored object.

**Connecting.** The topbar's *API key* dialog stores a key in `sessionStorage`, so it dies with the tab and is never written into the image. `GET /api/v1/me` then tells the portal what that key may do, which is what drives navigation — Documents and Search are disabled without `knowledge:read`, History and Favorites without `chat:completions`. That filtering is convenience only: the backend authorizes every request regardless of what the UI chose to show. Settings lists **every** scope the platform defines, granted or not, because "you do not have this" is the answer somebody goes there for.

This is a development bridge. Once the Identity Service issues JWTs to the portal, the browser stops holding a platform credential; until then, connect the narrowest key that does the job and keep `admin:keys` out of the browser.

**Chat.** The workspace streams over SSE, picks its model list from `/api/v1/models` (defaulting to whatever the gateway calls default, never a hard-coded model name), and shows real token counts and latency per turn. `EventSource` cannot send an `Authorization` header, so the stream is read with `fetch` and reassembled across network chunks.

**Documents.** Upload asks only for a title, a classification and the text: company and department come from the credential, so a caller cannot file a document outside its own scope, and the classification select offers only levels at or below its clearance. The list polls while anything is `pending` or `processing`, because ingestion is asynchronous and a successful upload is not yet a searchable document.

**Search.** Two modes over the same corpus and the same ACL predicates: passages (pgvector cosine fused with PostgreSQL full-text) and relationships (the graph, ACL filtered at every hop). Both print the classifications the answer was drawn from — an empty result with no clearance line is indistinguishable from an empty corpus.

**Favorites.** `GET/PUT/DELETE /api/v1/favorites` marks assistants, conversations and documents, gated by `chat:completions` for the same reason history is: a caller's own marks are part of using the platform, not a separate capability. The server resolves each mark against its record using the predicates the owning endpoint uses and drops the ones it can no longer read — verified by raising a favourited document to `restricted` against a `confidential` key, watching the mark disappear from `GET /api/v1/favorites`, restoring the classification and watching it return, with the row never deleted.

**Live figures.** The Developer Portal stat cards read environment, version, capabilities, model availability and compute status from the API. Only the roadmap remains local state — it is a planning artefact, not platform data.

**Languages.** Every string the platform owns goes through `t(key)` in Thai, English, Chinese, Burmese and Japanese, with `formatDate`/`formatNumber` following the active language (ARCHITECTURE-v1 section 10). The choice is stored per browser and defaults to the closest match from `navigator.languages`; `<html lang>` follows it so screen readers and hyphenation behave.

The catalogue is typed from the English keys, so **a missing translation in any language fails the build** rather than silently falling back — verified by deleting one Burmese string and watching `tsc` reject it:

```
src/i18n.ts(636,7): error TS2741: Property '"chat.error.catalogue.title"' is missing
  in type '{ ... }' but required in type 'Catalogue'.
```

Two bodies of text are deliberately **not** translated and are labelled as such in the UI: the twelve governance rules with the ten stack entries, and the roadmap's phase and milestone names. They mirror `docs/ARCHITECTURE-v1.md` and should be translated together with it, by a reviewer who can check the terminology. The Chinese, Japanese and Burmese UI strings are a first pass and want a native review before production.

Type checking is part of the build: `npm run build` runs `tsc --noEmit` first, so a type error fails the image. Dependencies are pinned by `package-lock.json` and installed with `npm ci`.

## After changing source

Both images build their artefacts at image build time — there is no bind mount or dev server, so edits are invisible until the image is rebuilt:

```powershell
docker compose up -d --build portal   # after editing web/
docker compose up -d --build api      # after editing control-plane/
```

## Architecture boundary

`portal -> AI Gateway / Orchestrator -> authorized tools and compute providers`

ERP and users must never address the Compute Plane directly. Production placement puts `api`, portal, PostgreSQL, and Redis in VM4, while Ollama/vLLM runs on GPU-passthrough VM5 over a private network.
