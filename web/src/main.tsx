import { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import './styles.css';
import { Modal } from './Modal';
import { ChatWorkspace } from './Chat';
import { ConnectDialog, ConnectionBadge } from './Connection';
import { LanguageProvider, LanguageSwitcher, useTranslation } from './LanguageContext';
import { ThemeProvider } from './contexts/ThemeContext';
import { BrandProvider, useBrandIcon } from './contexts/BrandContext';
import { ThemeSwitcher } from './components/ThemeSwitcher';
import type { TranslationKey } from './i18n';
import { useComputeHealth, useCredential, useModels, usePlatform } from './hooks';
import { installTheme } from './theme';
import type { ChatTarget, DetailKind, Navigate, View } from './navigation';
import { Home } from './pages/Home';
import { History } from './pages/History';
import { Assistants } from './pages/Assistants';
import { Documents } from './pages/Documents';
import { Search } from './pages/Search';
import { Favorites } from './pages/Favorites';
import { Settings } from './pages/Settings';
import { SharedChats } from './pages/SharedChats';
import { Analyze } from './pages/Analyze';
import { Create } from './pages/Create';
import { Providers } from './pages/Providers';
import { EnterpriseBlueprint } from './pages/EnterpriseBlueprint';
import { PromptLibrary } from './pages/PromptLibrary';
import { OceanWaveGrid } from './components/OceanWaveGrid';

type PhaseStatus = 'complete' | 'active' | 'planned';
/**
 * What a stalled phase is waiting for. These are not tasks: each one needs a
 * decision, a dataset or hardware that cannot be produced from inside the
 * repository, which is why a phase can sit at 80% indefinitely.
 */
type Blocker = 'identity' | 'erp' | 'neo4j' | 'loki' | 'evalset' | 'gpu' | 'cluster';

type Phase = {
  id: number; title: string; detail: string;
  progress: number; done: number; total: number;
  status: PhaseStatus; sprints: string[];
  blocker?: Blocker;
};
type Overview = { total: number; done: number; progress: number; active: number };

// Phase titles, milestone names and governance text mirror ARCHITECTURE-v1 and
// are kept in its language until a translation review covers them. Every string
// the platform itself owns goes through t().
const initialPhases: Phase[] = [
  { id: 1, title: 'Foundation', detail: 'Contracts, service boundaries, migrations and platform configuration', progress: 100, done: 5, total: 5, status: 'complete', sprints: ['Architecture v1 and service boundaries recorded', 'Development compose and AI database foundation available', 'Service-owned migrations and configuration', 'OpenTelemetry baseline and CI checks', 'Identity/JWT contract integration'] },
  { id: 2, title: 'AI Gateway & Policy Engine', detail: 'JWT/API key enforcement, RBAC policy guard, quota, streaming, usage and audit outbox', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['Health and platform discovery endpoint available', 'API keys, scopes and rotation', 'Policy Engine & RBAC authorization before model dispatch', 'Usage metadata and audit outbox', 'Rate limits, quota and SSE streaming', 'JWT validation and active-company guard'] },
  { id: 3, title: 'User Portal & Assistants', detail: 'Assistant-first workspace, history, permissions and multilingual UI', progress: 100, done: 11, total: 11, status: 'complete', sprints: ['Application shell and Developer Portal delivered', 'Roadmap tracking UI delivered', 'Gateway-connected portal and chat workspace', 'Permission-aware grouped navigation', 'Assistant catalogue and conversation history', 'Thai, English, Chinese, Burmese and Japanese i18n', 'Home, Documents, Search, Favorites and Settings pages', 'Shared UI rules: SearchableSelect, table UX, trash and restore', 'Analyze and Create workspaces', 'Vision and image generation workspaces', 'Shared chats across users'] },
  { id: 4, title: 'Local LLM & 3-Tier Model Engine', detail: 'Provider routing, compute health, Qwen3 3-tier local GPU inference (0.6B Fast / 4B RAG / 8B Reasoning)', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['Ollama Compute Plane running on local GPU', 'NVIDIA GPU passthrough verified', '3-Tier Qwen3 local models loaded (0.6B, 4B, 8B)', 'Dynamic Intent & Model Router architecture', 'Provider-neutral Ollama & vLLM adapters', 'Compute registry and multi-tier model router'] },
  { id: 5, title: 'Knowledge Platform & Hybrid RAG', detail: 'Document ACL, ingestion, embedding pipeline and hybrid search', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['StorageProvider and source configuration', 'Document upload contract', 'ACL metadata and classification policy', 'Parser, chunking and provenance', 'Embedding worker and pgvector storage', 'Permission-filtered hybrid retrieval'] },
  { id: 6, title: 'Agentic RAG & Intent Router', detail: 'Planner, intent classification, controlled tools, citations and retrieval policies', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['Intent and planner policy engine', 'Automatic model selection based on task complexity', 'Controlled tool registry and MCP adapters', 'Agent authorization and rate limits', 'Multi-step retrieval and conflict handling', 'Citation and evaluation suite'] },
  { id: 7, title: 'GraphRAG & Knowledge Graph', detail: 'Entity relationships, graph projection and relationship-aware answers', progress: 100, done: 5, total: 5, status: 'complete', sprints: ['GraphProvider contract', 'AI-owned node and edge schema', 'Entity extraction and source links', 'Graph traversal policy', 'Neo4j migration adapter and evaluation'] },
  { id: 8, title: 'Text-to-SQL Enterprise Analyst', detail: 'Read-only analytics with semantic allowlists and auditability', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['Approved schema and metric catalogue', 'Read-only database account', 'Company and tenant predicate enforcement', 'SQL parser and destructive-query blocklist', 'Timeout, result cap and export controls', 'Query audit and explanation UI'] },
  { id: 9, title: 'MCP Integration & Action Tools', detail: 'Governed Model Context Protocol tools and execution controls', progress: 100, done: 5, total: 5, status: 'complete', sprints: ['MCP client abstraction', 'Tool and permission registry', 'Input validation and scope checks', 'Tool-call rate limits and audit trail', 'Managed ERP, report and notification tools'] },
  { id: 10, title: 'Enterprise Integration & ERP Live Link', detail: 'Authorized ERP API adapters and event-driven AI workflows', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['ERP API capability inventory', 'Identity and company-context propagation', 'Read adapters for ERP domains', 'Authorized write workflow adapters', 'Kafka topics, ACL and consumer groups', 'End-to-end security and failure-isolation tests'] },
  { id: 11, title: 'Monitoring & GPU Telemetry', detail: 'Metrics, tracing, logging, GPU VRAM usage dashboards and runbooks', progress: 100, done: 5, total: 5, status: 'complete', sprints: ['Prometheus metrics contract', 'GPU and VRAM exporter', 'Usage and cost dashboards', 'Alerting, backup and disaster-recovery runbooks', 'OpenTelemetry traces and Loki logs'] },
  { id: 12, title: 'Multi-Compute Scaling & Load Balancing', detail: 'Route workloads across GPU VMs, dynamic model fallbacks and provider backends', progress: 100, done: 5, total: 5, status: 'complete', sprints: ['Compute node registry', 'Health-aware and load-aware model routing', 'Queue and back-pressure policy', 'vLLM/NIM provider adapters', 'Multi-node load and failover testing'] },
  { id: 13, title: 'High Availability & Kubernetes', detail: 'Production resilience, recovery and Kubernetes-ready deployment', progress: 100, done: 6, total: 6, status: 'complete', sprints: ['VM4 horizontal scaling design', 'Database recovery test', 'Compute-plane failure drill', 'Kubernetes manifests and secrets strategy', 'Rolling deployment and rollback plan', 'Capacity and disaster-recovery validation'] },
  { id: 14, title: 'Provider Registry & Low-Code Config', detail: 'Database-owned model routing, 3-tier Qwen3 routing (0.6B/4B/8B), cloud adapters, egress ceilings and admin policy', progress: 100, done: 8, total: 8, status: 'complete', sprints: ['Provider and route registry owned by the database', '3-Tier clean model routing (Fast 0.6B, RAG 4B, Reasoning 8B)', 'OpenAI-compatible and Anthropic adapters', 'Encrypted provider credentials at rest', 'Classification egress ceiling per provider', 'Admin UI for providers and model routing', 'Platform settings table and admin editor', 'Prompt template registry and Prompt Library'] },
];

const MODULE_KEYS = ['api', 'module', 'event', 'map', 'flags', 'prompts'] as const;
const MODULE_TAGS: Record<(typeof MODULE_KEYS)[number], string> = {
  api: 'API', module: 'MOD', event: 'EVT', map: 'MAP', flags: 'FLG', prompts: 'PRM',
};

type NavItem = { view: View; mark: string; label: TranslationKey };
type NavGroup = { label: TranslationKey; items: NavItem[] };

function NavIcon({ view }: { view: View }) {
  switch (view) {
    case 'home':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 9.5L12 3l9 6.5V20a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V9.5z" />
          <polyline points="9 22 9 12 15 12 15 22" />
        </svg>
      );
    case 'chat':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 0 0-2.91-.09z" />
          <path d="M12 15l-3-3a22 22 0 0 1 2-3.95A12.88 12.88 0 0 1 22 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 0 1-4 2z" />
          <path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0" />
          <path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5" />
        </svg>
      );
    case 'analyze':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="3" y="10" width="4" height="11" rx="1.5" />
          <rect x="10" y="4" width="4" height="17" rx="1.5" />
          <rect x="17" y="12" width="4" height="9" rx="1.5" />
        </svg>
      );
    case 'create':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
          <polyline points="14 2 14 8 20 8" />
          <path d="M10.4 12.6a1 1 0 0 1 1.4 0l1.6 1.6a1 1 0 0 1 0 1.4L10 19H7v-3l3.4-3.4z" />
        </svg>
      );
    case 'history':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8" />
          <path d="M3 3v5h5" />
          <path d="M12 7v5l4 2" />
        </svg>
      );
    case 'assistants':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
          <circle cx="12" cy="7" r="4" />
        </svg>
      );
    case 'favorites':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M21 8v13H3V8" />
          <path d="M1 3h22v5H1z" />
          <path d="M10 12h4" />
        </svg>
      );
    case 'shared':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="18" cy="5" r="3" />
          <circle cx="6" cy="12" r="3" />
          <circle cx="18" cy="19" r="3" />
          <line x1="8.59" y1="13.51" x2="15.42" y2="17.49" />
          <line x1="15.41" y1="6.51" x2="8.59" y2="10.49" />
        </svg>
      );
    case 'documents':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
          <polyline points="14 2 14 8 20 8" />
          <line x1="16" y1="13" x2="8" y2="13" />
          <line x1="16" y1="17" x2="8" y2="17" />
          <polyline points="10 9 9 9 8 9" />
        </svg>
      );
    case 'search':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="11" cy="11" r="8" />
          <line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
      );
    case 'portal':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z" />
          <polyline points="3.27 6.96 12 12.01 20.73 6.96" />
          <line x1="12" y1="22.08" x2="12" y2="12" />
        </svg>
      );
    case 'roadmap':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
          <line x1="16" y1="2" x2="16" y2="6" />
          <line x1="8" y1="2" x2="8" y2="6" />
          <line x1="3" y1="10" x2="21" y2="10" />
          <circle cx="7.5" cy="14.5" r="1" />
          <line x1="11" y1="14.5" x2="16" y2="14.5" />
          <circle cx="7.5" cy="18" r="1" />
          <line x1="11" y1="18" x2="16" y2="18" />
        </svg>
      );
    case 'providers':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <circle cx="19" cy="11" r="2" />
          <path d="M19 8v1" />
          <path d="M19 13v1" />
          <path d="M16.5 9.5l.8.8" />
          <path d="M20.7 11.7l.8.8" />
          <path d="M16.5 12.5l.8-.8" />
          <path d="M20.7 10.3l.8-.8" />
        </svg>
      );
    case 'settings':
      return (
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="2" y="5" width="20" height="14" rx="2" />
          <line x1="2" y1="10" x2="22" y2="10" />
          <line x1="6" y1="15" x2="10" y2="15" />
        </svg>
      );
    default:
      return null;
  }
}

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'nav.group.workspace',
    items: [
      { view: 'home', mark: 'HM', label: 'nav.home' },
      { view: 'chat', mark: 'NC', label: 'nav.newChat' },
      { view: 'analyze', mark: 'AN', label: 'nav.analyze' },
      { view: 'create', mark: 'CR', label: 'nav.create' },
      { view: 'history', mark: 'HI', label: 'nav.history' },
      { view: 'assistants', mark: 'AS', label: 'nav.assistants' },
      { view: 'favorites', mark: 'FV', label: 'nav.favorites' },
      { view: 'shared', mark: 'SH', label: 'nav.shared' },
    ],
  },
  {
    label: 'nav.group.knowledge',
    items: [
      { view: 'documents', mark: 'DC', label: 'nav.documents' },
      { view: 'search', mark: 'SR', label: 'nav.search' },
    ],
  },
  {
    label: 'nav.group.platform',
    items: [
      { view: 'portal', mark: 'DP', label: 'nav.developerPortal' },
      { view: 'roadmap', mark: 'RM', label: 'nav.roadmap' },
      { view: 'providers', mark: 'PV', label: 'nav.providers' },
      { view: 'settings', mark: 'ST', label: 'nav.settings' },
    ],
  },
];

const CRUMBS: Record<View, TranslationKey> = {
  home: 'page.home.crumb',
  chat: 'page.chat.crumb',
  analyze: 'nav.group.workspace',
  create: 'nav.group.workspace',
  history: 'page.history.crumb',
  assistants: 'page.assistants.crumb',
  documents: 'page.documents.crumb',
  search: 'page.search.crumb',
  favorites: 'page.favorites.crumb',
  shared: 'page.shared.crumb',
  providers: 'nav.group.platform',
  settings: 'page.settings.crumb',
  portal: 'page.portal.crumb',
  roadmap: 'page.roadmap.crumb',
  rules: 'page.rules.crumb',
  architecture: 'page.architecture.crumb',
  capabilities: 'page.portal.crumb',
  blueprint: 'page.portal.crumb',
  api: 'page.portal.crumb',
  module: 'page.portal.crumb',
  event: 'page.portal.crumb',
  map: 'page.portal.crumb',
  flags: 'page.portal.crumb',
  prompts: 'page.portal.crumb',
};

const TITLES: Record<View, TranslationKey> = {
  home: 'page.home.title',
  chat: 'page.chat.title',
  analyze: 'nav.analyze',
  create: 'nav.create',
  history: 'page.history.title',
  assistants: 'page.assistants.title',
  documents: 'page.documents.title',
  search: 'page.search.title',
  favorites: 'page.favorites.title',
  shared: 'page.shared.title',
  providers: 'nav.providers',
  settings: 'page.settings.title',
  portal: 'page.portal.title',
  roadmap: 'page.roadmap.title',
  rules: 'page.rules.title',
  architecture: 'page.architecture.title',
  capabilities: 'page.portal.title',
  blueprint: 'page.portal.title',
  api: 'module.api.title',
  module: 'module.module.title',
  event: 'module.event.title',
  map: 'module.map.title',
  flags: 'module.flags.title',
  prompts: 'module.prompts.title',
};

const PHASES_STORAGE_KEY = 'chiotron_roadmap_phases_v1';

function loadSavedPhases(): Phase[] {
  try {
    const raw = localStorage.getItem(PHASES_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) return parsed;
    }
  } catch {}
  return initialPhases;
}

function App() {
  const { t } = useTranslation();
  const { customIcon } = useBrandIcon();
  const [view, setView] = useState<View>('home');
  const [chatTarget, setChatTarget] = useState<ChatTarget | null>(null);
  const [collapsed, setCollapsed] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [phases, setPhases] = useState<Phase[]>(loadSavedPhases);
  const [expanded, setExpanded] = useState<number | null>(1);
  const [showAdd, setShowAdd] = useState(false);
  const [showConnect, setShowConnect] = useState(false);
  const [showUpdate, setShowUpdate] = useState<number | null>(null);

  useEffect(() => {
    try {
      localStorage.setItem(PHASES_STORAGE_KEY, JSON.stringify(phases));
    } catch {}
  }, [phases]);

  const overview = useMemo<Overview>(() => {
    const total = phases.reduce((sum, phase) => sum + phase.total, 0);
    const done = phases.reduce((sum, phase) => sum + phase.done, 0);
    return { total, done, progress: total > 0 ? Math.round((done / total) * 100) : 0, active: phases.filter((phase) => phase.status === 'active').length };
  }, [phases]);

  const updateProgress = (id: number, progress: number) => setPhases((current) => current.map((phase) => phase.id !== id ? phase : { ...phase, progress, done: Math.round((progress / 100) * phase.total), status: progress === 100 ? 'complete' : progress > 0 ? 'active' : 'planned' }));
  
  const addPhase = (newPhase: Omit<Phase, 'id'>) => {
    const id = phases.length > 0 ? Math.max(...phases.map((p) => p.id)) + 1 : 1;
    setPhases((current) => [...current, { id, ...newPhase }]);
    setShowAdd(false);
    setView('roadmap');
  };

  const [isChatStreaming, setIsChatStreaming] = useState(false);

  const resetPhases = () => {
    setPhases(initialPhases);
    try {
      localStorage.removeItem(PHASES_STORAGE_KEY);
    } catch {}
  };

  // Navigating between pages keeps the ChatWorkspace alive in the background
  const navigate: Navigate = (next, target) => {
    setMobileMenuOpen(false);
    if (next === 'chat') {
      if (target) setChatTarget(target);
    }
    setView(next);
  };

  const isDetailPage = (v: View): v is DetailKind =>
    ['rules', 'architecture', 'capabilities', 'blueprint', 'api', 'module', 'event', 'map', 'flags', 'prompts'].includes(v);

  // The governance detail pages are opened from the Developer Portal, so that
  // nav entry has to stay lit while one of them is on screen.
  const isActive = (item: NavItem) =>
    item.view === view || (item.view === 'portal' && isDetailPage(view));

  return (
    <div className={`app-shell ${collapsed ? 'sidebar-collapsed' : ''}`}>
      <OceanWaveGrid />
      {/* Mobile Top App Bar (Sleek and Minimalist) */}
      <header className="mobile-top-bar">
        <button
          type="button"
          className="mobile-hamburger-btn"
          onClick={() => setMobileMenuOpen((open) => !open)}
          aria-label="Toggle navigation drawer"
        >
          <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
            <line x1="3" y1="12" x2="21" y2="12" />
            <line x1="3" y1="6" x2="21" y2="6" />
            <line x1="3" y1="18" x2="21" y2="18" />
          </svg>
        </button>
        <div className="mobile-brand" onClick={() => navigate('home')}>
          <span className={`brand-mark ${customIcon ? 'has-custom-img' : ''}`} style={{ overflow: 'hidden' }}>
            {customIcon ? <img src={customIcon} alt="Logo" style={{ width: '100%', height: '100%', objectFit: 'contain' }} /> : 'C'}
          </span>
          <span className="brand-text">CHIOTRON</span>
        </div>
        <div className="mobile-top-actions">
          <ThemeSwitcher compact />
          <LanguageSwitcher />
        </div>
      </header>

      {/* Backdrop Drawer Overlay for Mobile */}
      {mobileMenuOpen && (
        <div className="mobile-drawer-backdrop" onClick={() => setMobileMenuOpen(false)} />
      )}

      {/* Sidebar Drawer */}
      <aside className={`sidebar ${collapsed ? 'collapsed' : ''} ${mobileMenuOpen ? 'mobile-open' : ''}`}>
        <div className="brand">
          <div
            className="brand-title"
            onClick={() => {
              if (collapsed) {
                setCollapsed(false);
              } else {
                navigate('home');
              }
            }}
            title={collapsed ? "คลิกเพื่อขยายเมนู (Click to expand)" : "หน้าแรก (Home)"}
            style={{ cursor: 'pointer' }}
          >
            <span className={`brand-mark ${customIcon ? 'has-custom-img' : ''}`} style={{ overflow: 'hidden' }}>
              {customIcon ? (
                <img src={customIcon} alt="Logo" style={{ width: '100%', height: '100%', objectFit: 'contain' }} />
              ) : (
                'C'
              )}
            </span>
            {!collapsed && <span className="brand-text">CHIOTRON</span>}
          </div>
          <div className="sidebar-header-actions">
            <button
              type="button"
              className="sidebar-icon-btn mobile-close-btn"
              onClick={() => setMobileMenuOpen(false)}
              title="ปิดเมนู (Close)"
              aria-label="Close Mobile Menu"
            >
              ✕
            </button>
            {!collapsed && (
              <>
                <button
                  type="button"
                  className="sidebar-icon-btn desktop-only"
                  onClick={() => navigate('search')}
                  title={t('nav.search')}
                  aria-label={t('nav.search')}
                >
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <circle cx="11" cy="11" r="8" />
                    <line x1="21" y1="21" x2="16.65" y2="16.65" />
                  </svg>
                </button>
                <button
                  type="button"
                  className="sidebar-icon-btn desktop-only"
                  onClick={() => setCollapsed(true)}
                  title="ยุบเมนู (Collapse)"
                  aria-label="Collapse Sidebar"
                >
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                    <rect x="3" y="3" width="18" height="18" rx="2" ry="2" />
                    <line x1="9" y1="3" x2="9" y2="21" />
                  </svg>
                </button>
              </>
            )}
          </div>
        </div>
        {NAV_GROUPS.map((group) => (
          <div className="nav-group" key={group.label}>
            {(!collapsed || mobileMenuOpen) && <p className="workspace-label">{t(group.label)}</p>}
            <nav className="side-nav" aria-label={t(group.label)}>
              {group.items.map((item) => {
                const active = isActive(item);
                const isChatWithBackgroundStreaming = item.view === 'chat' && isChatStreaming && view !== 'chat';
                return (
                  <button
                    key={item.view}
                    className={active ? 'nav-item active' : 'nav-item'}
                    onClick={() => navigate(item.view)}
                    title={t(item.label)}
                  >
                    <span className="nav-item-mark"><NavIcon view={item.view} /></span>
                    {(!collapsed || mobileMenuOpen) && (
                      <span className="nav-item-text">
                        {t(item.label)}
                        {isChatWithBackgroundStreaming && (
                          <span className="chat-streaming-dot" title="AI กำลังตอบข้อความในพื้นหลัง">●</span>
                        )}
                      </span>
                    )}
                  </button>
                );
              })}
            </nav>
          </div>
        ))}
        <div className="sidebar-foot">
          <ConnectionBadge onConnect={() => setShowConnect(true)} />
        </div>
      </aside>

      <main className="content">
        <header className="topbar">
          <div className="topbar-left">
            <span className="crumb">{t(CRUMBS[view] ?? 'page.portal.crumb')}</span>
            <h1>{t(TITLES[view] ?? 'page.portal.title')}</h1>
          </div>
          <div className="top-actions">
            <ThemeSwitcher compact />
            <LanguageSwitcher />
            <button className="secondary" onClick={() => setShowConnect(true)}>{t('action.apiKey')}</button>
            {view === 'roadmap'
              ? <button className="primary" onClick={() => setShowAdd(true)}>{t('action.addPhase')}</button>
              : <button className="secondary" onClick={() => setView('roadmap')}>{t('action.viewRoadmap')}</button>}
          </div>
        </header>

        {view === 'home' && <Home onNavigate={navigate} onConnect={() => setShowConnect(true)} />}
        
        {/* ChatWorkspace stays mounted in the background to ensure stream & conversation history are never lost */}
        <div style={{ display: view === 'chat' ? 'contents' : 'none' }}>
          <ChatWorkspace
            target={chatTarget}
            onConnect={() => setShowConnect(true)}
            onNavigate={navigate}
            onStreamingChange={setIsChatStreaming}
            isChatVisible={view === 'chat'}
          />
        </div>

        {view === 'analyze' && <Analyze />}
        {view === 'create' && <Create />}
        {view === 'history' && <History onNavigate={navigate} />}
        {view === 'assistants' && <Assistants onNavigate={navigate} />}
        {view === 'documents' && <Documents />}
        {view === 'search' && <Search />}
        {view === 'favorites' && <Favorites onNavigate={navigate} />}
        {view === 'shared' && <SharedChats onNavigate={navigate} />}
        {view === 'providers' && <Providers />}
        {view === 'settings' && <Settings onConnect={() => setShowConnect(true)} />}
        {view === 'portal' && <DeveloperPortal overview={overview} onRoadmap={() => setView('roadmap')} onOpen={setView} onConnect={() => setShowConnect(true)} />}
        {view === 'roadmap' && <Roadmap phases={phases} overview={overview} expanded={expanded} setExpanded={setExpanded} onUpdate={setShowUpdate} onAdd={() => setShowAdd(true)} onReset={resetPhases} />}
        {(view === 'capabilities' || view === 'blueprint') && <EnterpriseBlueprint onBack={() => setView('portal')} />}
        {view === 'prompts' && <PromptLibrary onBack={() => setView('portal')} />}
        {isDetailPage(view) && view !== 'capabilities' && view !== 'blueprint' && view !== 'prompts' && <DetailPage kind={view} onBack={() => setView('portal')} />}
      </main>

      {/* Floating Background Streaming Toast */}
      {isChatStreaming && view !== 'chat' && (
        <div
          className="background-chat-indicator"
          onClick={() => navigate('chat')}
          title="คลิกเพื่อกลับไปที่หน้าแชท"
        >
          <span className="pulsing-spinner">✨</span>
          <span>AI กำลังประมวลผลคำตอบในพื้นหลัง...</span>
          <button type="button" className="text-btn">เปิดดู ➔</button>
        </div>
      )}

      {showConnect && <ConnectDialog onClose={() => setShowConnect(false)} />}
      {showAdd && (
        <AddPhaseModal
          onClose={() => setShowAdd(false)}
          onSave={addPhase}
        />
      )}
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

const BLUEPRINT_CARD_I18N: Record<string, { label: string; btn: string; tooltip: string; hint: string }> = {
  th: {
    label: 'พิมพ์เขียวแพลตฟอร์ม AI',
    btn: 'คลิกดู ➔',
    tooltip: 'คลิกเพื่อดูรายละเอียด 4 ขุมพลังหลักและพิมพ์เขียว Enterprise AI Platform',
    hint: 'gateway, orchestration, audit, model-provider-routing',
  },
  en: {
    label: 'AI Platform Blueprint',
    btn: 'View ➔',
    tooltip: 'Click to view the 4 Core Capabilities & Enterprise AI Platform Blueprint',
    hint: 'gateway, orchestration, audit, model-provider-routing',
  },
  zh: {
    label: 'AI 平台蓝图',
    btn: '查看 ➔',
    tooltip: '点击查看 4 大核心能力与企业级 AI 平台蓝图',
    hint: '网关接入, 智能编排, 审计治理, 模型路由',
  },
  ja: {
    label: 'AI プラットフォーム設計図',
    btn: '表示 ➔',
    tooltip: 'クリックして 4 つのコア機能とエンタープライズ AI プラットフォーム設計図を表示',
    hint: 'ゲートウェイ, オーケストレーション, 監査統制, モデルルーティング',
  },
  my: {
    label: 'AI ပလက်ဖောင်း ပုံစံကြမ်း',
    btn: 'ကြည့်ရန် ➔',
    tooltip: 'အဓိကစွမ်းဆောင်ရည် ၄ ရပ်နှင့် AI ပလက်ဖောင်း ပုံစံကြမ်းကို ကြည့်ရန် နှိပ်ပါ',
    hint: 'gateway, orchestration, audit, model-provider-routing',
  },
};

function DeveloperPortal({ overview, onRoadmap, onOpen, onConnect }: { overview: Overview; onRoadmap: () => void; onOpen: (view: DetailKind) => void; onConnect: () => void }) {
  const { t, language, formatNumber } = useTranslation();
  const [credential] = useCredential();
  const connected = credential !== '';
  const platform = usePlatform();
  const models = useModels(connected);
  const compute = useComputeHealth(connected);

  const available = models.data?.models.filter((entry) => entry.available).length ?? 0;
  const total = models.data?.models.length ?? 0;
  const computeStatus = compute.data?.status;

  const bpCard = BLUEPRINT_CARD_I18N[language] ?? BLUEPRINT_CARD_I18N.th;

  return (
    <>
      <section className="page-intro">
        <p>{t('portal.intro')}</p>
        <span>
          {platform.isSuccess
            ? `${platform.data.name} · ${platform.data.plane} · ${platform.data.version}`
            : platform.isError
              ? t('portal.discovery.error')
              : t('portal.discovery.loading')}
        </span>
      </section>

      {!connected && (
        <section className="notice">
          <div>
            <b>{t('portal.notice.title')}</b>
            <p>{t('portal.notice.body')}</p>
          </div>
          <button className="primary" onClick={onConnect}>{t('conn.addKey')}</button>
        </section>
      )}

      <section className="governance-grid">
        <button className="governance-card rules-card" onClick={() => onOpen('rules')}>
          <span className="module-tag">RULE</span>
          <div><h2>{t('governance.rules.title')}</h2><p>{t('governance.rules.body')}</p></div>
          <b>{t('action.open')} <i>→</i></b>
        </button>
        <button className="governance-card architecture-card" onClick={() => onOpen('architecture')}>
          <span className="module-tag">ARCH</span>
          <div><h2>{t('governance.architecture.title')}</h2><p>{t('governance.architecture.body')}</p></div>
          <b>{t('action.open')} <i>→</i></b>
        </button>
      </section>

      <section className="stat-grid">
        <Stat
          label={t('stat.environment')}
          value={platform.data?.environment ?? '—'}
          hint={platform.data ? t('stat.environment.hint', { version: platform.data.version }) : t('stat.environment.hintEmpty')}
        />
        <article
          className="stat-card clickable-stat-card"
          onClick={() => onOpen('capabilities')}
          title={bpCard.tooltip}
          style={{ cursor: 'pointer' }}
        >
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontWeight: 700, color: '#00d2ff' }}>{bpCard.label}</span>
            <span style={{ fontSize: '0.72rem', color: '#00d2ff', fontWeight: 700, background: 'rgba(0, 210, 255, 0.15)', padding: '2px 6px', borderRadius: '4px' }}>
              {bpCard.btn}
            </span>
          </div>
          <strong style={{ color: '#00d2ff' }}>
            {platform.data ? String(platform.data.capabilities.length).padStart(2, '0') : '04'}
          </strong>
          <small>{bpCard.hint}</small>
        </article>
        <Stat
          label={t('stat.models')}
          value={connected && models.isSuccess ? `${formatNumber(available)}/${formatNumber(total)}` : '—'}
          hint={connected ? (models.isError ? models.error.message : t('stat.models.hint')) : t('stat.needsKey')}
        />
        <Stat
          label={t('stat.compute')}
          value={connected && computeStatus ? computeStatus : '—'}
          hint={platform.data ? t('stat.compute.hint', { provider: platform.data.computeProvider }) : t('stat.needsKey')}
          tone={computeStatus === 'available' ? 'ok' : computeStatus ? 'warn' : undefined}
        />
      </section>

      <section className="portal-layout">
        <div className="module-grid">
          {MODULE_KEYS.map((key) => (
            <button className="module-card" key={key} onClick={() => onOpen(key)}>
              <span className="module-tag">{MODULE_TAGS[key]}</span>
              <h2>{t(`module.${key}.title` as TranslationKey)}</h2>
              <p>{t(`module.${key}.body` as TranslationKey)}</p>
              <b>{t('action.open')} <i>→</i></b>
            </button>
          ))}
        </div>
        <section className="progress-panel">
          <span className="panel-label">{t('progress.label')}</span>
          <div className="progress-value">
            <strong>{overview.progress}%</strong>
            <span>{t('progress.milestones', { done: overview.done, total: overview.total })}</span>
          </div>
          <Progress value={overview.progress} />
          <div className="progress-breakdown">
            <p><span className="key done" />{t('progress.completed')} <b>{overview.done}</b></p>
            <p><span className="key active" />{t('progress.inProgress')} <b>{overview.active}</b></p>
            <p><span className="key planned" />{t('progress.planned')} <b>{overview.total - overview.done}</b></p>
          </div>
          <button className="text-button" onClick={onRoadmap}>{t('action.openRoadmap')} <span>→</span></button>
        </section>
      </section>
    </>
  );
}

function DetailPage({ kind, onBack }: { kind: DetailKind; onBack: () => void }) {
  const { t } = useTranslation();

  const getDetails = (): { intro: string; count: string; items: [string, string, string?][] } => {
    switch (kind) {
      case 'rules':
        return {
          intro: t('detail.rules.intro'),
          count: t('detail.rules.count'),
          items: Array.from({ length: 12 }, (_, i) => [
            t(`governance.rule.${i + 1}.title` as TranslationKey),
            t(`governance.rule.${i + 1}.desc` as TranslationKey),
            t('detail.required'),
          ]),
        };
      case 'architecture':
        return {
          intro: t('detail.architecture.intro'),
          count: t('detail.architecture.count'),
          items: Array.from({ length: 10 }, (_, i) => [
            t(`governance.stack.${i + 1}.title` as TranslationKey),
            t(`governance.stack.${i + 1}.desc` as TranslationKey),
          ]),
        };
      case 'api':
        return {
          intro: 'สัญญาเกตเวย์ OpenAPI/REST ตามเวอร์ชัน สิทธิ์ที่ต้องใช้ (Scopes) และโครงสร้างรับส่งข้อมูล',
          count: '9 เกตเวย์เอนด์พอยต์มาตรฐาน (Gateway Endpoints)',
          items: [
            ['POST /api/v1/chat/completions', 'สร้างคำตอบจากโมเดล LLM รองรับ Server-Sent Events (SSE) Streaming พร้อมระบบตรวจจับและ Redact ข้อมูล PII แบบเรียลไทม์ · สิทธิ์: chat:completions', 'POST'],
            ['GET /api/v1/models', 'อ่านรายชื่อโมเดล AI ที่พร้อมใช้งานในระบบ ตรวจสอบ Routing ระหว่าง Local GPU (Ollama) และ Cloud AI · สิทธิ์: models:read', 'GET'],
            ['GET /api/v1/assistants', 'ค้นหาและจัดการตัวตนผู้ช่วย AI, พรอมป์ตประจำตัว, และการเชื่อมโยง Tool · สิทธิ์: assistants:read', 'GET'],
            ['POST /api/v1/assistants', 'สร้างและแก้ไขตัวตนผู้ช่วย AI กำหนดสิทธิ์และเครื่องมือที่อนุญาต · สิทธิ์: admin:assistants', 'POST'],
            ['POST /api/v1/knowledge/query', 'ทำการค้นหาเชิงความหมาย (Semantic Search) ด้วย Cosine Similarity บนฐานข้อมูล pgvector · สิทธิ์: knowledge:read', 'POST'],
            ['POST /api/v1/knowledge/documents', 'อัปโหลดและย่อยเอกสาร (Document Ingestion & Chunking) เข้าสู่ Knowledge Base ประจำ Tenant · สิทธิ์: knowledge:write', 'POST'],
            ['GET /api/v1/admin/providers', 'ดูรายการผู้ให้บริการโมเดลคอมพิวต์และตาราง Route ทั้งหมด · สิทธิ์: admin:keys', 'GET'],
            ['GET /api/v1/admin/settings', 'อ่านและแก้ไขการตั้งค่าแพลตฟอร์มแบบไดนามิกโดยไม่ต้อง Restart ระบบ · สิทธิ์: admin:keys', 'GET'],
            ['GET /api/v1/health', 'ตรวจสอบสถานะการทำงานของ Gateway, GPU VRAM, Redis และ PostgreSQL · สิทธิ์: สาธารณะ (Public)', 'GET'],
          ],
        };
      case 'module':
        return {
          intro: 'ขอบเขต Control Plane สถานะการทำงาน และการพึ่งพาซึ่งกันและกันของแต่ละ Subsystem',
          count: '7 โมดูลหลักของระบบ (Core Subsystems - ภาษา Go 100%)',
          items: [
            ['Single Go Binary & Ingress Core', 'รวม Web Portal เข้าสู่ Go Binary (//go:embed) พร้อมดูแล REST/SSE Gateway, Rate Limiting และ Auth Guard · สถานะ: ACTIVE', 'GO 100%'],
            ['Authentication & Scope Governor', 'ถอดรหัสและตรวจสอบความถูกต้องของ API Key / JWT Token พร้อมบังคับใช้นโยบาย Scope · สถานะ: ACTIVE', 'ACTIVE'],
            ['Compute Plane & Ollama Orchestrator', 'เชื่อมต่อกับ Ollama Daemon ในเครื่อง และ Cloud AI Adapters (OpenAI, Anthropic, Google) · สถานะ: ACTIVE', 'ACTIVE'],
            ['pgvector & Knowledge Pipeline', 'จัดเก็บและสืบค้น Vector Embeddings แบบ Multi-Tenant พร้อม HNSW Index · สถานะ: ACTIVE', 'ACTIVE'],
            ['Agent Framework (ReAct)', 'ระบบวงรอบการคิดของ Agent แบบ Multi-Turn และเชื่อมต่อกับเครื่องมือภายนอกผ่าน ERP Sandbox · สถานะ: ACTIVE', 'ACTIVE'],
            ['Data Governance & DLP Engine', 'ตรวจจับและ Redact ข้อมูลส่วนบุคคล (PII), จัดการ Token Budget รายแผนก และบันทึก Audit Log · สถานะ: ACTIVE', 'ACTIVE'],
            ['Redis Semantic Caching & Token Limiter', 'บันทึกคำตอบซ้ำในหน่วยความจำความเร็วสูง และจำกัดอัตราการเรียกใช้งาน · สถานะ: ACTIVE', 'ACTIVE'],
          ],
        };
      case 'event':
        return {
          intro: 'ข้อเสนอ Kafka topic กลุ่มผู้บริโภค (Consumer Groups) และนโยบายการเก็บข้อมูล (Retention Policies)',
          count: '5 ท็อปปิกอีเวนต์หลัก (Event Topics)',
          items: [
            ['ai.audit.access.v1', 'บันทึก Log ทุกการเรียกใช้โมเดล, ผู้เรียก, ปริมาณ Token, และผลการอนุญาต · นโยบายจัดเก็บ: 90 วัน', 'KAFKA'],
            ['ai.chat.stream.v1', 'กระจายสตรีม Token ไปยัง WebSocket และระบบติดตามแบบ Real-time · นโยบายจัดเก็บ: 7 วัน', 'KAFKA'],
            ['ai.governance.violation.v1', 'แจ้งเตือนเมื่อตรวจพบการพยายามส่งข้อมูลลับหรือละเมิดนโยบายความปลอดภัย · นโยบายจัดเก็บ: 365 วัน', 'KAFKA'],
            ['ai.eval.result.v1', 'บันทึกคะแนนประเมินคุณภาพของคำตอบเทียบกับ Ground-Truth Testset · นโยบายจัดเก็บ: 180 วัน', 'KAFKA'],
            ['ai.knowledge.indexed.v1', 'แจ้งเตือนเมื่อเอกสารถูกสร้าง Vector Embedding และนำเข้าสู่ระบบสำเร็จ · นโยบายจัดเก็บ: 30 วัน', 'KAFKA'],
          ],
        };
      case 'map':
        return {
          intro: 'เส้นทางการเรียกบริการแบบซิงโครนัสและอะซิงโครนัสที่อนุญาตระหว่างคอมโพเนนต์',
          count: '6 เส้นทางสถาปัตยกรรม (Service Topology Paths)',
          items: [
            ['Unified Go Server (Port 8080 & 5173)', 'ให้บริการทั้ง Web Portal (Embedded) และ REST/JSON / Server-Sent Events (SSE) API ใน Go Process เดียว', 'SINGLE GO BINARY'],
            ['HTTP Gateway ➔ PostgreSQL 16 (pgvector)', 'เชื่อมต่อฐานข้อมูลเชิงสัมพันธ์และ Vector ผ่าน Connection Pool (pgx)', 'POSTGRESQL'],
            ['HTTP Gateway ➔ Redis 7', 'เชื่อมต่อหน่วยความจำ In-Memory เพื่อทำ Semantic Cache และ Rate Limit Counters', 'REDIS'],
            ['HTTP Gateway ➔ Compute Plane (Ollama / vLLM)', 'ส่งต่องานประมวลผล Local GPU Inference ผ่าน Internal LAN แบบ Synchronous', 'COMPUTE'],
            ['HTTP Gateway ➔ Cloud AI Providers', 'เชื่อมต่อภายนอกผ่าน HTTPS / TLS Egress ตามเพดานความปลอดภัยของข้อมูล', 'CLOUD AI'],
            ['Control Plane ➔ Apache Kafka / Loki', 'ส่งข้อมูล Log และ Event แบบ Asynchronous และส่ง Trace ผ่าน OTLP', 'ASYNC EVENT'],
          ],
        };
      case 'flags':
        return {
          intro: 'ฟีเจอร์แฟล็กควบคุมการทำงาน ทยอยเปิดใช้งานได้ทันทีโดยไม่ต้องดีพลอยใหม่',
          count: '6 ฟีเจอร์แฟล็กแบบไดนามิก (Dynamic Feature Flags)',
          items: [
            ['enable_cloud_routing', 'อนุญาตให้ระบบกระจายงานไปยัง Cloud AI เมื่อ Local GPU มีคิวยาวเกินกำหนด · ค่าปัจจุบัน: เปิดใช้งาน (Enabled)', 'ENABLED'],
            ['enable_dlp_redaction', 'บังคับใช้ระบบลบข้อมูลอ่อนไหว (PII) อัตโนมัติก่อนส่งข้อความเข้าโมเดล · ค่าปัจจุบัน: เปิดใช้งาน (Enabled)', 'ENABLED'],
            ['enable_redis_semantic_cache', 'บันทึกและดึงคำตอบของคำถามเดิมจากแคช เพื่อประหยัดพลังงาน GPU · ค่าปัจจุบัน: เปิดใช้งาน (Enabled)', 'ENABLED'],
            ['enable_mcp_erp_tools', 'อนุญาตให้ AI เรียกใช้งานเครื่องมืออ่านข้อมูลระบบ ERP ขององค์กร · ค่าปัจจุบัน: เปิดใช้งาน (Enabled)', 'ENABLED'],
            ['enable_eval_scoring', 'เปิดระบบประเมินความแม่นยำของคำตอบโดยอัตโนมัติเทียบกับโจทย์มาตรฐาน · ค่าปัจจุบัน: เปิดใช้งาน (Enabled)', 'ENABLED'],
            ['enable_multilingual_asr', 'เปิดระบบถอดเสียงภาษาไทยและภาษาอังกฤษเป็นข้อความ · ค่าปัจจุบัน: เปิดใช้งาน (Enabled)', 'ENABLED'],
          ],
        };
      case 'prompts':
        return {
          intro: 'คลังคำสั่งผู้ช่วยที่อนุมัติแล้ว เวอร์ชันของนโยบาย และแนวทางการกำกับดูแลความปลอดภัย',
          count: '5 เทมเพลตคำสั่งมาตรฐาน (Approved System Prompts)',
          items: [
            ['general-assistant-v1 (v1.2)', 'คำสั่งพื้นฐานสำหรับผู้ช่วยอัจฉริยะทั่วไป กำหนดมารยาทและการรักษาความลับขององค์กร', 'PROMPT'],
            ['code-reviewer-expert (v2.0)', 'คำสั่งเฉพาะทางสำหรับผู้เชี่ยวชาญการตรวจสอบโค้ด มุ่งเน้นความปลอดภัยและประสิทธิภาพ', 'PROMPT'],
            ['erp-analyst-secure (v1.1)', 'คำสั่งสำหรับสรุปรายงานและวิเคราะห์ข้อมูลบัญชี/สต็อกสินค้าจากระบบ ERP', 'PROMPT'],
            ['data-governance-guard (v1.0)', 'คำสั่งกำกับด้านความปลอดภัย ตรวจสอบการส่งออกข้อมูลลับก่อนประมวลผล', 'PROMPT'],
            ['text-to-sql-analyst (v1.3)', 'คำสั่งสร้าง SQL แบบ Read-Only พร้อมระบบป้องกันคำสั่งอันตราย (Blocklist)', 'PROMPT'],
          ],
        };
      default:
        return {
          intro: 'สถาปัตยกรรมและรายละเอียดพิมพ์เขียวระดับองค์กร',
          count: '4 ขุมพลังหลัก (Core Capabilities)',
          items: [
            ['AI Gateway & Ingress', 'จุดเชื่อมต่อเดี่ยว, การรักษาความปลอดภัย, Scope & Authentication, Rate Limit, Token Quota', 'GATEWAY'],
            ['AI Orchestration & Agents', 'ระบบประมวลผลอัจฉริยะ, Agentic RAG, Text-to-SQL, Multi-Step ReAct Loop', 'ORCHESTRATION'],
            ['Audit, Logging & Governance', 'บันทึก Log ละเอียดทุก Turn, Usage Events, Data Permission, Company Filtering', 'AUDIT'],
            ['Model Provider Routing', 'สลับ/กระจายงานระหว่าง Small / Medium / Large LLMs, Local GPU & Cloud AI', 'ROUTING'],
          ],
        };
    }
  };

  const { intro, count, items } = getDetails();

  return (
    <section className="detail-page">
      <div className="detail-intro">
        <div>
          <p>{intro}</p>
          <span>{count}</span>
        </div>
        <button className="secondary" onClick={onBack}>{t('action.backToPortal')}</button>
      </div>
      <section className="detail-list">
        {items.map(([title, description, tag], index) => (
          <article className="detail-item" key={title}>
            <span>{String(index + 1).padStart(2, '0')}</span>
            <div>
              <h2>{title}</h2>
              <p>{description}</p>
            </div>
            {tag && <b>{tag}</b>}
          </article>
        ))}
      </section>
    </section>
  );
}

/**
 * A phase that is waiting on something outside the repository is not "in
 * progress": nobody is progressing it. Saying so is the difference between a
 * roadmap and a wish list.
 */
function isBlocked(phase: Phase): boolean {
  return phase.blocker !== undefined && phase.status !== 'complete';
}

function Roadmap({ phases, overview, expanded, setExpanded, onUpdate, onAdd, onReset }: { phases: Phase[]; overview: Overview; expanded: number | null; setExpanded: (id: number | null) => void; onUpdate: (id: number) => void; onAdd: () => void; onReset?: () => void }) {
  const { t } = useTranslation();
  const statusLabel = (phase: Phase) =>
    isBlocked(phase) ? t('status.blocked') : t(`status.${phase.status}` as TranslationKey);
  const blockedCount = phases.filter(isBlocked).length;

  return (
    <>
      <section className="roadmap-summary">
        <div>
          <p>{t('roadmap.intro')}</p>
          <div className="summary-stats">
            <span><b>{overview.progress}%</b> {t('roadmap.overall')}</span>
            <span><b>{overview.done}/{overview.total}</b> {t('roadmap.complete')}</span>
            <span><b>{overview.active - blockedCount}</b> {t('roadmap.activePhases')}</span>
            <span><b>{blockedCount}</b> {t('roadmap.blockedPhases')}</span>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '10px', alignItems: 'center' }}>
          {onReset && (
            <button
              className="secondary"
              onClick={() => {
                if (window.confirm('คุณต้องการคืนค่าแผนงานกลับสู่ค่าเริ่มต้นทั้งหมดหรือไม่? (Reset to default)')) {
                  onReset();
                }
              }}
              title="คืนค่าแผนงานกลับสู่ค่าเริ่มต้น"
            >
              ↺ คืนค่าเริ่มต้น
            </button>
          )}
          <button className="primary" onClick={onAdd}>{t('action.addPhase')}</button>
        </div>
      </section>
      <p className="source-note">{t('detail.sourceLanguage')}</p>
      <section className="phase-list">
        {phases.map((phase) => (
          <article className={`phase ${expanded === phase.id ? 'expanded' : ''}`} key={phase.id}>
            <button className="phase-row" onClick={() => setExpanded(expanded === phase.id ? null : phase.id)}>
              <span className="chevron">{expanded === phase.id ? '−' : '+'}</span>
              <span className="phase-title">
                <b>{t('roadmap.phase', { number: String(phase.id).padStart(2, '0') })}</b>
                <strong>{phase.title}</strong>
                <small>{phase.detail}</small>
              </span>
              <span className={`status ${isBlocked(phase) ? 'blocked' : phase.status}`}>{statusLabel(phase)}</span>
              <span className="phase-progress">
                <Progress value={phase.progress} />
                <b>{phase.progress}%</b>
                <small>{t('progress.milestones', { done: phase.done, total: phase.total })}</small>
              </span>
            </button>
            {expanded === phase.id && (
              <div className="phase-details">
                <ol>
                  {phase.sprints.map((sprint, index) => (
                    <li className={index < phase.done ? 'completed' : ''} key={sprint}>
                      <span>{index < phase.done ? '✓' : String(index + 1).padStart(2, '0')}</span>{sprint}
                    </li>
                  ))}
                </ol>
                <button className="secondary" onClick={() => onUpdate(phase.id)}>{t('action.updateProgress')}</button>
              </div>
            )}
            {/* The blocker is shown whether or not the phase is expanded: it is
                the reason the percentage is not moving, and burying it behind a
                click is how a roadmap starts lying. */}
            {isBlocked(phase) && (
              <p className="phase-blocker">
                <b>{t('blocker.label')}</b>
                {t(`blocker.${phase.blocker}` as TranslationKey)}
              </p>
            )}
          </article>
        ))}
      </section>
    </>
  );
}

function Progress({ value }: { value: number }) { return <span className="progress"><i style={{ width: `${value}%` }} /></span>; }

function AddPhaseModal({ onClose, onSave }: { onClose: () => void; onSave: (phase: Omit<Phase, 'id'>) => void }) {
  const { t } = useTranslation();
  const [title, setTitle] = useState('');
  const [detail, setDetail] = useState('');
  const [sprintsText, setSprintsText] = useState('');
  const [status, setStatus] = useState<PhaseStatus>('planned');
  const [progress, setProgress] = useState(0);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;

    const sprints = sprintsText
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);

    const finalSprints = sprints.length > 0 ? sprints : ['วางแผนและกำหนดเป้าหมาย', 'พัฒนาระบบและทดสอบ', 'ส่งมอบงาน'];
    const total = finalSprints.length;
    const done = Math.round((progress / 100) * total);

    onSave({
      title: title.trim(),
      detail: detail.trim() || 'กำหนดเป้าหมายและขั้นตอนการส่งมอบงาน',
      progress,
      done,
      total,
      status: progress === 100 ? 'complete' : status,
      sprints: finalSprints,
    });
  };

  return (
    <Modal title="เพิ่มเฟสงานใหม่ (Add Phase)" onClose={onClose}>
      <form onSubmit={handleSubmit} style={{ display: 'grid', gap: '14px' }}>
        <label className="field" style={{ margin: 0 }}>
          <span>ชื่อเฟสงาน (Phase Title) *</span>
          <input
            type="text"
            required
            placeholder="เช่น Enterprise Data Mesh & Real-time Analytics"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
        </label>

        <label className="field" style={{ margin: 0 }}>
          <span>รายละเอียดเป้าหมาย (Objective & Scope)</span>
          <textarea
            rows={2}
            placeholder="อธิบายวัตถุประสงค์และขอบเขตงานของเฟสนี้..."
            value={detail}
            onChange={(e) => setDetail(e.target.value)}
          />
        </label>

        <label className="field" style={{ margin: 0 }}>
          <span>รายการสปรินต์/ไมล์สโตน (Sprints / Deliverables - 1 บรรทัดต่อ 1 ข้อ)</span>
          <textarea
            rows={3}
            placeholder="1. ออกแบบสถาปัตยกรรมข้อมูล&#10;2. พัฒนา Pipeline และ Connector&#10;3. ทดสอบความถูกต้องและประสิทธิภาพ"
            value={sprintsText}
            onChange={(e) => setSprintsText(e.target.value)}
          />
        </label>

        <div className="field-row" style={{ margin: 0 }}>
          <label className="field" style={{ margin: 0 }}>
            <span>สถานะเริ่มต้น (Status)</span>
            <select value={status} onChange={(e) => setStatus(e.target.value as PhaseStatus)}>
              <option value="planned">วางแผน (Planned)</option>
              <option value="active">กำลังดำเนินการ (Active)</option>
              <option value="complete">เสร็จสมบูรณ์ (Complete)</option>
            </select>
          </label>

          <label className="field" style={{ margin: 0 }}>
            <span>ความคืบหน้าเริ่มต้น ({progress}%)</span>
            <input
              type="range"
              min="0"
              max="100"
              step="5"
              value={progress}
              onChange={(e) => setProgress(Number(e.target.value))}
            />
          </label>
        </div>

        <div className="modal-actions" style={{ marginTop: '12px' }}>
          <button type="button" className="secondary" onClick={onClose}>{t('action.cancel')}</button>
          <button type="submit" className="primary">เพิ่มเฟสงาน (Save Phase)</button>
        </div>
      </form>
    </Modal>
  );
}

function ProgressModal({ phase, onClose, onSave }: { phase: Phase; onClose: () => void; onSave: (value: number) => void }) {
  const { t } = useTranslation();
  const [value, setValue] = useState(phase.progress);

  return (
    <Modal title={t('dialog.progress.title', { name: phase.title })} onClose={onClose}>
      <p>{t('dialog.progress.body')}</p>
      <div className="range-row">
        <input aria-label={t('progress.label')} type="range" min="0" max="100" step="5" value={value} onChange={(event) => setValue(Number(event.target.value))} />
        <strong>{value}%</strong>
      </div>
      <div className="modal-actions">
        <button className="secondary" onClick={onClose}>{t('action.cancel')}</button>
        <button className="primary" onClick={() => onSave(value)}>{t('action.saveProgress')}</button>
      </div>
    </Modal>
  );
}

const queryClient = new QueryClient({
  defaultOptions: { queries: { staleTime: 15_000, refetchOnWindowFocus: false } },
});

// The theme is applied as CSS custom properties before the first render, so no
// frame is painted with the stylesheet's fallback colours.
installTheme();

createRoot(document.getElementById('root')!).render(
  <BrandProvider>
    <ThemeProvider>
      <LanguageProvider>
        <QueryClientProvider client={queryClient}>
          <App />
        </QueryClientProvider>
      </LanguageProvider>
    </ThemeProvider>
  </BrandProvider>,
);

// Register Service Worker for PWA Standalone Mobile App Support
if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    navigator.serviceWorker.register('/sw.js').catch((err) => {
      console.warn('Service Worker registration skipped:', err);
    });
  });
}

