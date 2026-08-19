# Production Resilience, Disaster Recovery & Failure Drill Runbook

## 1. Overview & Architecture Isolation Principles

The AI Platform operates across two distinct failure domains:
1. **VM4 Control Plane**: Stateless Go API services, Gateway, Knowledge ACL, and Web Portal.
2. **VM5 / GPU Compute Plane**: State-heavy local LLM inference engines (Ollama, vLLM, NVIDIA NIM).

> [!IMPORTANT]
> **Rule 8 (Independent failure domains)**: Losing the GPU Compute Plane (VM5) must only degrade inference capabilities with clear `503 Service Unavailable` error responses; it must never take down the Control Plane (VM4), authentication, database, or portal.

---

## 2. PostgreSQL Recovery & Point-In-Time Recovery (PITR) Drill

### 2.1 Backup Strategy
- **Continuous WAL Archiving**: PostgreSQL WAL files are streamed to S3-compatible cold storage.
- **Nightly Base Backups**: pg_dumpall / pg_basebackup snapshot taken at 02:00 UTC.

### 2.2 Recovery Verification Steps
1. Spin up an isolated recovery instance:
   ```bash
   docker run -d --name pg-restore -e POSTGRES_PASSWORD=recovery -p 5433:5432 pgvector/pgvector:pg17
   ```
2. Restore the latest base backup:
   ```bash
   pg_restore -h localhost -p 5433 -U postgres -d ai_platform /backups/latest.dump
   ```
3. Verify table counts, vector indices, and tenant constraints:
   ```sql
   SELECT count(*) FROM knowledge_documents;
   SELECT count(*) FROM knowledge_chunks;
   SELECT count(*) FROM platform_settings;
   ```

---

## 3. Compute-Plane Failure Drill (VM5 Down Simulation)

### 3.1 Simulation
Simulate a catastrophic loss of GPU compute node by terminating the Ollama/vLLM daemon:
```bash
docker stop ceap-ollama-1
```

### 3.2 Expected Platform Behavior
- **Health Check (`/ready`)**: Continues reporting HTTP 200 for the Control Plane.
- **Chat Completions**: Requests targeting local models return structured JSON error `{"error": "compute provider unavailable"}`.
- **Failover**: If a secondary cloud provider is configured (e.g. OpenAI/Claude fallback), requests automatically fail over smoothly.
- **Web Portal**: Displays an amber warning status on the compute health badge without UI crashing.

---

## 4. Kubernetes Rolling Deployments & Zero-Downtime Rollback

### 4.1 Rolling Update Strategy
Deployments use `maxSurge: 1` and `maxUnavailable: 0` ensuring zero downtime:
```bash
kubectl apply -k infra/k8s/
kubectl rollout status deployment/ai-control-plane -n ai-platform
```

### 4.2 Emergency Rollback
If application telemetry detects an elevated error rate post-deployment:
```bash
kubectl rollout undo deployment/ai-control-plane -n ai-platform
```

---

## 5. Capacity & High-Availability Verification

- **Horizontal Pod Autoscaling (HPA)**: Automatically scales Control Plane from 2 to 10 replicas when CPU exceeds 70% or memory exceeds 80%.
- **Pod Anti-Affinity & Topology Spread**: Pods are distributed across different physical hosts / availability zones.
