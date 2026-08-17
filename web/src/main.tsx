import { useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import './styles.css';
import { Modal } from './Modal';
import { ChatWorkspace } from './Chat';
import { ConnectDialog, ConnectionBadge, SCOPE_ASSISTANTS_READ, SCOPE_CHAT } from './Connection';
import { useComputeHealth, useCredential, useModels, usePlatform, useScopes } from './hooks';

type PhaseStatus = 'complete' | 'active' | 'planned';
type Phase = { id: number; title: string; detail: string; progress: number; done: number; total: number; status: PhaseStatus; sprints: string[] };
type View = 'portal' | 'roadmap' | 'rules' | 'architecture' | 'chat';
type Overview = { total: number; done: number; progress: number; active: number };

const initialPhases: Phase[] = [
  { id: 1, title: 'Foundation', detail: 'Contracts, service boundaries, migrations and platform configuration', progress: 80, done: 4, total: 5, status: 'active', sprints: ['Architecture v1 and service boundaries recorded', 'Development compose and AI database foundation available', 'Service-owned migrations and configuration', 'OpenTelemetry baseline and CI checks', 'Identity/JWT contract integration'] },
  { id: 2, title: 'AI Gateway', detail: 'JWT/API key enforcement, quota, streaming, usage and audit outbox', progress: 80, done: 4, total: 5, status: 'active', sprints: ['Health and platform discovery endpoint available', 'API keys, scopes and rotation', 'Usage metadata and audit outbox', 'Rate limits, quota and SSE streaming', 'JWT validation and active-company guard'] },
  { id: 3, title: 'User Portal', detail: 'Assistant-first workspace, history, permissions and multilingual UI', progress: 83, done: 5, total: 6, status: 'active', sprints: ['Application shell and Developer Portal delivered', 'Roadmap tracking UI delivered', 'Gateway-connected portal and chat workspace', 'Permission-aware navigation', 'Assistant catalogue and conversation history', 'Thai, English, Chinese, Burmese and Japanese i18n'] },
  { id: 4, title: 'Local LLM', detail: 'Provider routing, compute health and isolated local inference', progress: 100, done: 5, total: 5, status: 'complete', sprints: ['Ollama Compute Plane running', 'NVIDIA GPU passthrough verified', 'Qwen smoke-test model loaded', 'Provider-neutral Ollama adapter', 'Compute registry and model router'] },
  { id: 5, title: 'Knowledge Platform', detail: 'Document ACL, ingestion, embedding pipeline and hybrid search', progress: 0, done: 0, total: 6, status: 'planned', sprints: ['StorageProvider and source configuration', 'Document connector and upload contract', 'ACL metadata and classification policy', 'Parser, chunking and provenance', 'Embedding worker and pgvector storage', 'Permission-filtered hybrid retrieval'] },
  { id: 6, title: 'Agentic RAG', detail: 'Planner, controlled tools, citations and retrieval policies', progress: 0, done: 0, total: 5, status: 'planned', sprints: ['Intent and planner policy', 'Controlled tool registry', 'Agent authorization and rate limits', 'Multi-step retrieval and conflict handling', 'Citation and evaluation suite'] },
  { id: 7, title: 'GraphRAG', detail: 'Entity relationships, graph projection and relationship-aware answers', progress: 0, done: 0, total: 5, status: 'planned', sprints: ['GraphProvider contract', 'AI-owned node and edge schema', 'Entity extraction and source links', 'Graph traversal policy', 'Neo4j migration adapter and evaluation'] },
  { id: 8, title: 'Text-to-SQL', detail: 'Read-only analytics with semantic allowlists and auditability', progress: 0, done: 0, total: 6, status: 'planned', sprints: ['Approved schema and metric catalogue', 'Read-only database account', 'Company and tenant predicate enforcement', 'SQL parser and destructive-query blocklist', 'Timeout, result cap and export controls', 'Query audit and explanation UI'] },
  { id: 9, title: 'MCP Integration', detail: 'Governed Model Context Protocol tools and execution controls', progress: 0, done: 0, total: 5, status: 'planned', sprints: ['MCP client abstraction', 'Tool and permission registry', 'Input validation and scope checks', 'Tool-call rate limits and audit trail', 'Managed ERP, report and notification tools'] },
  { id: 10, title: 'Enterprise Integration', detail: 'Authorized ERP API adapters and event-driven AI workflows', progress: 0, done: 0, total: 6, status: 'planned', sprints: ['ERP API capability inventory', 'Identity and company-context propagation', 'Read adapters for ERP domains', 'Authorized write workflow adapters', 'Kafka topics, ACL and consumer groups', 'End-to-end security and failure-isolation tests'] },
  { id: 11, title: 'Monitoring & Operations', detail: 'Metrics, tracing, logging, usage dashboards and runbooks', progress: 0, done: 0, total: 5, status: 'planned', sprints: ['Prometheus metrics contract', 'GPU and VRAM exporter', 'OpenTelemetry traces and Loki logs', 'Usage and cost dashboards', 'Alerting, backup and disaster-recovery runbooks'] },
  { id: 12, title: 'Multi-Compute Scaling', detail: 'Route workloads across GPU VMs and provider backends', progress: 0, done: 0, total: 5, status: 'planned', sprints: ['Compute node registry', 'Health-aware model routing', 'Queue and back-pressure policy', 'vLLM/NIM provider adapters', 'Multi-node load and failover testing'] },
  { id: 13, title: 'High Availability & Kubernetes', detail: 'Production resilience, recovery and Kubernetes-ready deployment', progress: 0, done: 0, total: 6, status: 'planned', sprints: ['VM4 horizontal scaling design', 'Database recovery test', 'Compute-plane failure drill', 'Kubernetes manifests and secrets strategy', 'Rolling deployment and rollback plan', 'Capacity and disaster-recovery validation'] },
];

const PAGE_TITLES: Record<View, { crumb: string; title: string }> = {
  rules: { crumb: 'CORE GOVERNANCE', title: 'กฎหลักที่ต้องไม่หลุด' },
  architecture: { crumb: 'SYSTEM ARCHITECTURE', title: 'สถาปัตยกรรมระบบ' },
  roadmap: { crumb: 'ROADMAP', title: 'Roadmap' },
  chat: { crumb: 'WORKSPACE', title: 'Chat' },
  portal: { crumb: 'DEVELOPER PORTAL', title: 'Developer Portal' },
};

function App() {
  const [view, setView] = useState<View>('portal');
  const [phases, setPhases] = useState(initialPhases);
  const [expanded, setExpanded] = useState<number | null>(1);
  const [showAdd, setShowAdd] = useState(false);
  const [showConnect, setShowConnect] = useState(false);
  const [showUpdate, setShowUpdate] = useState<number | null>(null);
  const { has } = useScopes();
  // The workspace picks an assistant before it can send anything, so it needs
  // both scopes to be usable at all.
  const canChat = has(SCOPE_CHAT) && has(SCOPE_ASSISTANTS_READ);

  const overview = useMemo<Overview>(() => {
    const total = phases.reduce((sum, phase) => sum + phase.total, 0);
    const done = phases.reduce((sum, phase) => sum + phase.done, 0);
    return { total, done, progress: Math.round((done / total) * 100), active: phases.filter((phase) => phase.status === 'active').length };
  }, [phases]);

  const updateProgress = (id: number, progress: number) => setPhases((current) => current.map((phase) => phase.id !== id ? phase : { ...phase, progress, done: Math.round((progress / 100) * phase.total), status: progress === 100 ? 'complete' : progress > 0 ? 'active' : 'planned' }));
  const addPhase = () => { const id = Math.max(...phases.map((phase) => phase.id)) + 1; setPhases((current) => [...current, { id, title: 'New delivery phase', detail: 'Define the objective and delivery milestones.', progress: 0, done: 0, total: 3, status: 'planned', sprints: ['Define outcome', 'Implement', 'Validate'] }]); setShowAdd(false); setView('roadmap'); };

  const page = PAGE_TITLES[view];
  const portalActive = view === 'portal' || view === 'rules' || view === 'architecture';

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">C</span><span>CHIOTRON</span></div>
        <p className="workspace-label">AI PLATFORM</p>
        <nav className="side-nav" aria-label="Platform navigation">
          <button className={portalActive ? 'nav-item active' : 'nav-item'} onClick={() => setView('portal')}><span>DP</span>Developer Portal</button>
          <button className={view === 'roadmap' ? 'nav-item active' : 'nav-item'} onClick={() => setView('roadmap')}><span>RM</span>Roadmap</button>
          {/* Navigation follows the connected key's scopes. This is convenience
              only: the backend authorizes every request regardless. */}
          <button
            className={view === 'chat' ? 'nav-item active' : canChat ? 'nav-item' : 'nav-item muted'}
            onClick={() => canChat && setView('chat')}
            disabled={!canChat}
            title={canChat ? undefined : 'Needs a key with the chat:completions and assistants:read scopes'}
          ><span>CH</span>Chat</button>
          <button className="nav-item muted" disabled><span>KN</span>Knowledge</button>
        </nav>
        <div className="sidebar-foot">
          <ConnectionBadge onConnect={() => setShowConnect(true)} />
        </div>
      </aside>

      <main className="content">
        <header className="topbar">
          <div><span className="crumb">PLATFORM / {page.crumb}</span><h1>{page.title}</h1></div>
          <div className="top-actions">
            <button className="secondary" onClick={() => setShowConnect(true)}>API key</button>
            {view === 'roadmap'
              ? <button className="primary" onClick={() => setShowAdd(true)}>Add phase</button>
              : <button className="secondary" onClick={() => setView('roadmap')}>View roadmap</button>}
          </div>
        </header>

        {view === 'portal' && <DeveloperPortal overview={overview} onRoadmap={() => setView('roadmap')} onOpen={setView} onConnect={() => setShowConnect(true)} />}
        {view === 'roadmap' && <Roadmap phases={phases} overview={overview} expanded={expanded} setExpanded={setExpanded} onUpdate={setShowUpdate} onAdd={() => setShowAdd(true)} />}
        {view === 'chat' && <ChatWorkspace onConnect={() => setShowConnect(true)} />}
        {(view === 'rules' || view === 'architecture') && <DetailPage kind={view} onBack={() => setView('portal')} />}
      </main>

      {showConnect && <ConnectDialog onClose={() => setShowConnect(false)} />}
      {showAdd && <Modal title="Add roadmap phase" onClose={() => setShowAdd(false)}><p>A new planned phase with three delivery milestones will be added to the roadmap.</p><div className="modal-actions"><button className="secondary" onClick={() => setShowAdd(false)}>Cancel</button><button className="primary" onClick={addPhase}>Add phase</button></div></Modal>}
      {showUpdate !== null && <ProgressModal phase={phases.find((phase) => phase.id === showUpdate)!} onClose={() => setShowUpdate(null)} onSave={(progress) => { updateProgress(showUpdate, progress); setShowUpdate(null); }} />}
    </div>
  );
}

/** A stat card that shows live platform data, or a placeholder until one arrives. */
function Stat({ label, value, hint, tone }: { label: string; value: string; hint: string; tone?: string }) {
  return (
    <article className={tone ? `stat-card tone-${tone}` : 'stat-card'}>
      <span>{label}</span>
      <strong>{value}</strong>
      <small>{hint}</small>
    </article>
  );
}

function DeveloperPortal({ overview, onRoadmap, onOpen, onConnect }: { overview: Overview; onRoadmap: () => void; onOpen: (view: 'rules' | 'architecture') => void; onConnect: () => void }) {
  const [credential] = useCredential();
  const connected = credential !== '';
  const platform = usePlatform();
  const models = useModels(connected);
  const compute = useComputeHealth(connected);

  const available = models.data?.models.filter((entry) => entry.available).length ?? 0;
  const total = models.data?.models.length ?? 0;
  const computeStatus = compute.data?.status;
  const modules = [['API Registry', 'Versioned gateway contracts, scopes and owners', 'API'], ['Module Registry', 'Control Plane boundaries, health and dependencies', 'MOD'], ['Event Catalogue', 'Kafka topic proposals, groups and retention policy', 'EVT'], ['Dependency Map', 'Allowed synchronous and asynchronous service paths', 'MAP'], ['Feature Flags', 'Controlled rollout without redeployment', 'FLG'], ['Prompt Library', 'Approved assistant instructions and policy versions', 'PRM']];

  return (
    <>
      <section className="page-intro">
        <p>Architecture governance, API contracts and platform delivery.</p>
        <span>
          {platform.isSuccess
            ? `${platform.data.name} · ${platform.data.plane} plane · ${platform.data.version}`
            : platform.isError
              ? 'Control Plane unreachable'
              : 'Reading platform discovery…'}
        </span>
      </section>

      {!connected && (
        <section className="notice">
          <div>
            <b>Not connected to the Gateway.</b>
            <p>Platform discovery is public. Models, compute status and the chat workspace need an API key.</p>
          </div>
          <button className="primary" onClick={onConnect}>Add API key</button>
        </section>
      )}

      <section className="governance-grid">
        <button className="governance-card rules-card" onClick={() => onOpen('rules')}><span className="module-tag">RULE</span><div><h2>กฎหลักที่ต้องไม่หลุด</h2><p>ข้อบังคับของระบบ - ทุกฟีเจอร์ต้องยึดตาม</p></div><b>Open <i>→</i></b></button>
        <button className="governance-card architecture-card" onClick={() => onOpen('architecture')}><span className="module-tag">ARCH</span><div><h2>สถาปัตยกรรมระบบ</h2><p>Stack จริงที่กำลังรันอยู่ และ boundary ของแต่ละ plane</p></div><b>Open <i>→</i></b></button>
      </section>

      <section className="stat-grid">
        <Stat
          label="Environment"
          value={platform.data?.environment ?? '—'}
          hint={platform.data ? `version ${platform.data.version}` : 'from /api/v1/platform'}
        />
        <Stat
          label="Capabilities"
          value={platform.data ? String(platform.data.capabilities.length).padStart(2, '0') : '—'}
          hint={platform.data?.capabilities.join(', ') ?? 'declared by the Control Plane'}
        />
        <Stat
          label="Models"
          value={connected && models.isSuccess ? `${available}/${total}` : '—'}
          hint={connected ? (models.isError ? models.error.message : 'logical routes loaded') : 'connect a key to read'}
        />
        <Stat
          label="Compute plane"
          value={connected && computeStatus ? computeStatus : '—'}
          hint={platform.data ? `${platform.data.computeProvider} provider` : 'connect a key to read'}
          tone={computeStatus === 'available' ? 'ok' : computeStatus ? 'warn' : undefined}
        />
      </section>

      <section className="portal-layout">
        <div className="module-grid">{modules.map(([title, description, tag]) => <button className="module-card" key={title}><span className="module-tag">{tag}</span><h2>{title}</h2><p>{description}</p><b>Open <i>→</i></b></button>)}</div>
        <section className="progress-panel">
          <span className="panel-label">Delivery progress</span>
          <div className="progress-value"><strong>{overview.progress}%</strong><span>{overview.done}/{overview.total} milestones</span></div>
          <Progress value={overview.progress} />
          <div className="progress-breakdown">
            <p><span className="key done" />Completed <b>{overview.done}</b></p>
            <p><span className="key active" />In progress <b>{overview.active}</b></p>
            <p><span className="key planned" />Planned <b>{overview.total - overview.done}</b></p>
          </div>
          <button className="text-button" onClick={onRoadmap}>Open roadmap <span>→</span></button>
        </section>
      </section>
    </>
  );
}

function DetailPage({ kind, onBack }: { kind: 'rules' | 'architecture'; onBack: () => void }) {
  const rules = [
    ['Control / Compute boundary', 'VM4 decides, manages and secures. VM5 performs GPU inference only; normal users never reach a model endpoint directly.'],
    ['ERP remains system of record', 'AI orchestrates approved ERP APIs and never duplicates ERP business rules or writes ERP tables directly.'],
    ['Gateway is mandatory', 'Browser and external callers use the AI Gateway only. Provider credentials and API keys stay server-side.'],
    ['Authorization everywhere', 'RBAC + ABAC applies to assistants, agents, MCP, tools, knowledge, SQL, export and company context.'],
    ['Permission-aware retrieval', 'Document ACL metadata follows every chunk. Retrieval filters access before content reaches the LLM.'],
    ['Safe Text-to-SQL', 'Use a read-only account, semantic allowlists, company predicates, timeouts, result limits and immutable audit records.'],
    ['Provider abstraction', 'LLM, embedding, vector, graph, search, storage, queue, vision and speech providers are adapters, not business dependencies.'],
    ['Independent failure domains', 'AI failure must not affect ERP. VM5 loss leaves VM4 available; a compute node can be replaced without changing the portal.'],
    ['Service-owned data', 'AI owns its database/schema. Cross-service work uses APIs or Kafka events; normal records use soft delete.'],
    ['Observable and auditable', 'Every AI request, agent/tool call, SQL execution and configuration change creates usage metadata and audit events.'],
    ['Configurable policy', 'Limits, model routing, agent/RAG rules and retention policy are configuration, not hard-coded business constants.'],
    ['Consistent UI rules', 'Use the shared layout, modal dialogs, permission-aware UI, React Query, translated text and central theme configuration.'],
  ];
  const stack = [
    ['Portal', 'React 19, Vite, TypeScript and the unified AI workspace UI, calling the Gateway with a scoped API key.'],
    ['Control Plane', 'Go API on port 8080: configuration, migrations, API keys, rate limits, usage/audit outbox and the compute registry.'],
    ['Compute Plane', 'Ollama in an isolated Docker network with NVIDIA GPU passthrough; qwen2.5:0.5b is the current development smoke-test model.'],
    ['AI data', 'PostgreSQL 16 with pgvector; AI-owned schema applied by service migrations at startup.'],
    ['Cache and events', 'Redis 7 holds rate-limit counters under ai:*. Production reuses existing Redis namespaces and Kafka KRaft topics with ACLs.'],
    ['Identity', 'API keys today. Production integrates with the existing Identity Service JWT, roles, permissions and active-company scope.'],
    ['Observability', 'Prometheus scrapes /metrics; OpenTelemetry traces export to the existing Tempo when a collector is configured.'],
    ['Ingress and deployment', 'Existing Nginx handles TLS/ingress. VM4 and VM5 use separate Docker Compose deployments before a future Kubernetes migration.'],
    ['Storage and knowledge', 'StorageProvider allows local/NAS/S3/MinIO. pgvector is initial vector store; Qdrant, Milvus and Neo4j remain replaceable adapters.'],
    ['Operational guard', 'VM5 exposes no public port, model/provider access passes through VM4, and ERP always continues when Node 4 is unavailable.'],
  ];
  const items = kind === 'rules' ? rules : stack;
  const intro = kind === 'rules' ? 'These are non-negotiable guardrails. A feature that violates one must be redesigned before implementation.' : 'A truthful view of the local development stack and the enterprise services it is designed to integrate with.';
  return <section className="detail-page"><div className="detail-intro"><div><p>{intro}</p><span>{kind === 'rules' ? '12 mandatory engineering guardrails' : 'Development runtime plus production integration target'}</span></div><button className="secondary" onClick={onBack}>Back to Developer Portal</button></div><section className="detail-list">{items.map(([title, description], index) => <article className="detail-item" key={title}><span>{String(index + 1).padStart(2, '0')}</span><div><h2>{title}</h2><p>{description}</p></div>{kind === 'rules' && <b>Required</b>}</article>)}</section></section>;
}

function Roadmap({ phases, overview, expanded, setExpanded, onUpdate, onAdd }: { phases: Phase[]; overview: Overview; expanded: number | null; setExpanded: (id: number | null) => void; onUpdate: (id: number) => void; onAdd: () => void }) { return <><section className="roadmap-summary"><div><p>Track platform delivery by phase, milestone and operational readiness.</p><div className="summary-stats"><span><b>{overview.progress}%</b> overall delivery</span><span><b>{overview.done}/{overview.total}</b> milestones complete</span><span><b>{overview.active}</b> active phases</span></div></div><button className="primary" onClick={onAdd}>Add phase</button></section><section className="phase-list">{phases.map((phase) => <article className={`phase ${expanded === phase.id ? 'expanded' : ''}`} key={phase.id}><button className="phase-row" onClick={() => setExpanded(expanded === phase.id ? null : phase.id)}><span className="chevron">{expanded === phase.id ? '−' : '+'}</span><span className="phase-title"><b>Phase {String(phase.id).padStart(2, '0')}</b><strong>{phase.title}</strong><small>{phase.detail}</small></span><span className={`status ${phase.status}`}>{phase.status === 'complete' ? 'Complete' : phase.status === 'active' ? 'In progress' : 'Planned'}</span><span className="phase-progress"><Progress value={phase.progress} /><b>{phase.progress}%</b><small>{phase.done}/{phase.total} milestones</small></span></button>{expanded === phase.id && <div className="phase-details"><ol>{phase.sprints.map((sprint, index) => <li className={index < phase.done ? 'completed' : ''} key={sprint}><span>{index < phase.done ? '✓' : String(index + 1).padStart(2, '0')}</span>{sprint}</li>)}</ol><button className="secondary" onClick={() => onUpdate(phase.id)}>Update progress</button></div>}</article>)}</section></>; }
function Progress({ value }: { value: number }) { return <span className="progress"><i style={{ width: `${value}%` }} /></span>; }
function ProgressModal({ phase, onClose, onSave }: { phase: Phase; onClose: () => void; onSave: (value: number) => void }) { const [value, setValue] = useState(phase.progress); return <Modal title={`Update ${phase.title}`} onClose={onClose}><p>Set delivery progress for this phase. Completed milestones will be recalculated.</p><div className="range-row"><input aria-label="Phase progress" type="range" min="0" max="100" step="5" value={value} onChange={(event) => setValue(Number(event.target.value))} /><strong>{value}%</strong></div><div className="modal-actions"><button className="secondary" onClick={onClose}>Cancel</button><button className="primary" onClick={() => onSave(value)}>Save progress</button></div></Modal>; }

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 15_000, refetchOnWindowFocus: false } },
});

createRoot(document.getElementById('root')!).render(
  <QueryClientProvider client={queryClient}>
    <App />
  </QueryClientProvider>,
);
