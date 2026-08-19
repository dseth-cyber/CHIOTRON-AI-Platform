# 🏆 CHIOTRON ENTERPRISE AI PLATFORM (CEAP)
## Handover & Operations Guide (HANDOFF.md)
**Version:** 1.0 (Production-Ready)  
**Platform Status:** 100% Delivered (75 / 75 Milestones across 14 Phases)  
**Target Architecture:** Node 4 Dedicated GPU Node (VM4 Control Plane + VM5 Compute Plane)

---

## 1. Executive Summary

แพลตฟอร์ม **CHIOTRON Enterprise AI Platform (CEAP)** ได้รับการพัฒนา ตรวจทานความปลอดภัยตาม 12 Mandatory Engineering Guardrails และผ่านการทดสอบแบบ End-to-End ครบถ้วน 100% ตาม **Master Architecture Specification v1.0**

ระบบถูกสร้างขึ้นด้วยสถาปัตยกรรม **Hexagonal / Adapter Pattern** ที่แยกขาดระหว่าง **Control Plane (Stateless Orchestrator)** และ **Compute Plane (GPU Inferences)** โดยไม่สร้างภาระหรือ Single Point of Failure ให้กับระบบ ERP หลักตาม **Rule 8 (Independent failure domains)**

```
========================================================================================
🏆 MILESTONE PROGRESS OVERVIEW (100% COMPLETED)
========================================================================================
[Phase 01] Foundation ............................................. 100% (5/5) [COMPLETE]
[Phase 02] AI Gateway ............................................. 100% (5/5) [COMPLETE]
[Phase 03] User Portal ............................................ 100% (11/11) [COMPLETE]
[Phase 04] Local LLM .............................................. 100% (5/5) [COMPLETE]
[Phase 05] Knowledge Platform ..................................... 100% (6/6) [COMPLETE]
[Phase 06] Agentic RAG ............................................ 100% (5/5) [COMPLETE]
[Phase 07] GraphRAG ............................................... 100% (5/5) [COMPLETE]
[Phase 08] Text-to-SQL ............................................ 100% (6/6) [COMPLETE]
[Phase 09] MCP Integration ........................................ 100% (5/5) [COMPLETE]
[Phase 10] Enterprise Integration ................................. 100% (6/6) [COMPLETE]
[Phase 11] Monitoring & Operations ................................ 100% (5/5) [COMPLETE]
[Phase 12] Multi-Compute Scaling .................................. 100% (5/5) [COMPLETE]
[Phase 13] High Availability & Kubernetes ......................... 100% (6/6) [COMPLETE]
[Phase 14] Provider Registry & Low-Code Config .................... 100% (8/8) [COMPLETE]
========================================================================================
```

---

## 2. Directory Structure & Key Components

```
d:\codex\ceap
├── compose.yaml                    # Local multi-container compose environment
├── control-plane/                  # VM4 AI Control Plane (Go 1.25)
│   ├── cmd/api/main.go             # Application entrypoint & dependency injection
│   └── internal/
│       ├── agent/                  # Agent planner, citations & synthetic eval suite
│       ├── auth/                   # API keys, Enterprise JWT & Active-Company Guard
│       ├── compute/                # Model routing, vLLM/NIM scaling & back-pressure queue
│       ├── config/                 # Environment validation & secrets encryption
│       ├── enterprise/             # ERP Read/Write adapters & Kafka event publisher
│       ├── graph/                  # GraphRAG Cypher engine & Neo4j HTTP adapter
│       ├── httpapi/                # REST endpoints, SSE streaming & audit middleware
│       ├── knowledge/              # ACL chunking, pgvector embedding & hybrid search
│       ├── mcp/                    # Governed MCP client & Managed Mock ERP tools
│       ├── prompt/                 # Prompt template registry store & API
│       ├── settings/               # Platform settings store & dynamic configuration
│       ├── sqlguard/               # Governed Text-to-SQL parser & allowlist engine
│       ├── storage/                # StorageProvider (Local filesystem & S3/MinIO)
│       ├── telemetry/              # Prometheus metrics & Loki log shipper
│       └── tool/                   # Controlled Tool Registry & Scope enforcement
├── infra/
│   ├── k8s/                        # Kubernetes manifests (Deployment, HPA, Kustomize)
│   ├── postgres/                   # Database schemas & pgvector extensions
│   ├── prometheus/                 # Alert rules & Prometheus scrape configuration
│   └── grafana/                    # Operational & Cost dashboards
├── docs/
│   └── disaster-recovery.md        # Backup, PITR, Failure Drill & Rollback runbooks
├── scripts/
│   └── check.ps1                   # Full CI verification suite (Go + TypeScript + Compose)
└── web/                            # User Portal (React 19 + Vite 7 + Tailwind 3.4)
    └── src/
        ├── i18n.ts                 # 5-Language catalogue (TH, EN, ZH, JA, MY)
        ├── main.tsx                # Roadmap tracker & Layout shell
        └── pages/                  # Chat, Analyze, Create, Search, Settings, etc.
```

---

## 3. Operations & Quick Commands

### 3.1 การรันและทดสอบระบบทั้งหมด (CI Verification)
```powershell
powershell -File scripts/check.ps1
```
* รันการตรวจสอบ Go Module, `go vet`, Unit Tests ครบทุก Package
* ตรวจสอบ TypeScript Type Safety (`tsc --noEmit`) และ Vite Build
* ตรวจสอบความถูกต้องของ Docker Compose Configuration

### 3.2 การรัน Local Environment ผ่าน Docker Compose
```powershell
# รันระบบหลัก (PostgreSQL, Redis, API, Portal, GPU Worker)
docker compose up -d

# หากมีการแก้โค้ด Frontend/Backend ให้ Rebuild แบบ No-Cache
docker compose build --no-cache portal api; docker compose up -d
```

### 3.3 การเปิดใช้งาน Observability Profile (Prometheus + Grafana)
```powershell
docker compose --profile observability up -d
```
* **User Portal:** `http://localhost:5173`
* **API Swagger / Health:** `http://localhost:8080/health`
* **Prometheus Metrics:** `http://localhost:8080/metrics`
* **Grafana Dashboards:** `http://localhost:3000` (User: `admin`, Pass: `change-me-before-production`)

---

## 4. Production Connection Guide (การเชื่อมต่อระบบจริง)

เมื่อทีม Infrastructure หรือทีม ERP ส่งมอบ Credential ของระบบจริง ให้ตั้งค่าในไฟล์ `.env` หรือ Kubernetes Secret ดังนี้:

### 4.1 เชื่อมต่อ ERP Gateway จริง
```env
ERP_BASE_URL=http://erp-gateway.internal
ERP_TIMEOUT=15s
```
* โค้ดใน `control-plane/internal/enterprise/erp_adapter.go` จะสลับไปยิง REST API ของ ERP จริง พร้อมส่งต่อ Header `X-Company-ID`, `X-Caller-ID`, `X-Clearance` โดยอัตโนมัติ

### 4.2 เชื่อมต่อ Enterprise JWT / SSO
```env
JWT_SIGNING_SECRET=your-enterprise-identity-secret-key
JWT_ISSUER=https://identity.chiotron.com
```
* `auth.JWTValidator` จะตรวจสอบ Signature, Claims, Expiration และเปิดการทำงานของ `ActiveCompanyGuard`

### 4.3 เชื่อมต่อ S3 / MinIO Object Storage
```env
STORAGE_PROVIDER=s3
S3_ENDPOINT=https://s3.ap-southeast-1.amazonaws.com
S3_BUCKET=chiotron-knowledge-corpus
S3_REGION=ap-southeast-1
S3_ACCESS_KEY=AKIAXXXXXXXXXXXXXXXX
S3_SECRET_KEY=YYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYYY
S3_PATH_STYLE=false
```

### 4.4 เชื่อมต่อ Neo4j Graph Database
```env
NEO4J_ENDPOINT=http://neo4j-cluster.internal:7474
NEO4J_DATABASE=neo4j
NEO4J_USERNAME=neo4j
NEO4J_PASSWORD=your-neo4j-password
```

---

## 5. คำแนะนำและข้อเสนอแนะเชิงกลยุทธ์ (Strategic Recommendations)

1. **การทำ Fine-tuning / Model Evaluation (Continuous Optimization):**
   * ใช้ชุดทดสอบ `DefaultSyntheticEvalSet()` ใน `control-plane/internal/agent/eval.go` ในการวัดค่า Grounding Score และ Precision@K ทุกครั้งที่มีการเปลี่ยนเวอร์ชันของโมเดลท้องถิ่น (เช่น จาก Qwen 2.5 เป็นโมเดลที่ใหญ่ขึ้น)
2. **การขยาย GPU Compute Nodes (Multi-GPU Scaling):**
   * หากมีปริมาณการใช้งานในองค์กรเพิ่มขึ้น สามารถเพิ่มโหนด VM6/VM7 โดยต่อเข้ากับ `compute.NodeRegistry` ใน `internal/compute/scaling.go` ได้ทันที ระบบมี **Least-Loaded Load Balancer** และ **Back-Pressure Queue** รองรับการกระจายโหลดข้าม GPU อยู่แล้ว
3. **การรักษาความปลอดภัยของ Text-to-SQL:**
   * ตารางที่ต้องการให้ AI วิเคราะห์ควรเป็น View หรือ Table ใน Schema `analytics_*` เท่านั้น และรักษาการทำงานผ่าน `sqlguard.Engine` (Rule 6) เพื่อไม่ให้สืบค้นข้อมูลดิบของระบบหลักโดยตรง
4. **การสำรองข้อมูล (Disaster Recovery):**
   * ควรทำซ้อมกู้คืนฐานข้อมูล (PostgreSQL PITR Drill) ทุกไตรมาส ตามขั้นตอนที่ระบุใน [docs/disaster-recovery.md](file:///d:/codex/ceap/docs/disaster-recovery.md)

---
**Handover Completed by:** Antigravity AI Engineering Assistant  
**Date:** 2026-08-19
