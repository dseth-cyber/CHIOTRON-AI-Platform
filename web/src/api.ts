// Typed client for the AI Gateway.
//
// The portal talks to the Control Plane and nothing else; it never addresses a
// model provider directly (ARCHITECTURE-v1 section 2).

const BASE = (import.meta.env.VITE_API_BASE_URL ?? '').replace(/\/+$/, '');

// The credential lives in sessionStorage so it dies with the tab. This is the
// development bridge until the Identity Service issues JWTs to the portal; a
// key pasted here should carry only the scopes the operator actually needs.
const CREDENTIAL_KEY = 'ceap.apiKey';

export type Identity = {
  keyId: string;
  name: string;
  scopes: string[];
  companyId?: string;
  department?: string;
  /** The ABAC triple's third leg: how far up the classification ladder this key reads. */
  maxClassification: string;
  rateLimitPerMinute: number;
};

export type Platform = {
  name: string;
  plane: string;
  version: string;
  environment: string;
  capabilities: string[];
  computeProvider: string;
};

export type LogicalModel = {
  logical: string;
  provider: string;
  model: string;
  available: boolean;
  default?: boolean;
};

export type ModelCatalogue = { default: string; models: LogicalModel[] };

export type ProviderHealth = {
  status: string;
  latencyMs: number;
  error?: string;
  models?: { id: string; family?: string; parameterSize?: string; quantization?: string }[];
};

export type ComputeHealth = {
  status: 'available' | 'degraded' | 'unavailable';
  providers: Record<string, ProviderHealth>;
  time: string;
};

export type TokenUsage = { promptTokens: number; completionTokens: number; totalTokens: number };

/** Instructions are assistant policy; the catalogue never returns them. */
export type Assistant = {
  id: string;
  slug: string;
  name: string;
  description: string;
  logicalModel: string;
  temperature?: number;
  maxTokens?: number;
  companyId?: string;
  enabled: boolean;
};

export type ConversationSummary = {
  id: string;
  title: string;
  assistantId?: string;
  assistantSlug?: string;
  assistantName?: string;
  messageCount: number;
  totalTokens: number;
  createdAt: string;
  updatedAt: string;
};

export type StoredMessage = {
  role: 'user' | 'assistant';
  content: string;
  redacted?: boolean;
  promptTokens?: number;
  completionTokens?: number;
  model?: string;
  createdAt: string;
};

export type ConversationList = {
  conversations: ConversationSummary[];
  trash?: boolean;
  promptsPersisted: boolean;
};

export type ConversationDetail = {
  conversation: ConversationSummary;
  messages: StoredMessage[];
  promptsPersisted: boolean;
};

export type Document = {
  id: string;
  sourceSlug: string;
  title: string;
  mimeType: string;
  byteSize: number;
  checksum: string;
  classification: string;
  companyId?: string;
  department?: string;
  ownerId: string;
  /** pending, processing, ready or failed; the worker moves it along. */
  status: string;
  error?: string;
  chunkCount: number;
  ingestedAt?: string;
  createdAt: string;
};

export type DocumentList = {
  documents: Document[];
  /** Whether this listing is the trash, so a client cannot render one as the other. */
  trash?: boolean;
  /** Counts by status, so a page can say how much of the corpus is searchable. */
  status: Record<string, number>;
  readableClassifications: string[];
};

export type SearchHit = {
  chunkId: number;
  documentId: string;
  documentTitle: string;
  ordinal: number;
  content: string;
  classification: string;
  score: number;
  vectorScore?: number;
  keywordScore?: number;
};

export type SearchResult = { hits: SearchHit[]; readableClassifications: string[] };

export type GraphNode = {
  id: string;
  kind: string;
  name: string;
  normalisedName: string;
  classification: string;
  mentionCount: number;
};

export type GraphEdge = {
  sourceId: string;
  targetId: string;
  relation: string;
  weight: number;
  classification: string;
};

export type Subgraph = {
  seeds?: GraphNode[];
  nodes?: GraphNode[];
  edges?: GraphEdge[];
  truncated?: boolean;
};

export type FavoriteKind = 'assistant' | 'conversation' | 'document';

export type Favorite = {
  kind: FavoriteKind;
  targetId: string;
  label: string;
  detail?: string;
  createdAt: string;
};

export type ComputeProvider = {
  id: string;
  slug: string;
  name: string;
  description?: string;
  kind: string;
  baseUrl: string;
  hasCredential: boolean;
  credentialHint?: string;
  /** The most sensitive content that may be sent to this backend. */
  maxClassification: string;
  enabled: boolean;
  timeoutSeconds: number;
  lastCheckedAt?: string;
  lastStatus?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
};

export type ModelRoute = {
  id: string;
  logical: string;
  provider: string;
  model: string;
  default: boolean;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export type ProviderRegistry = {
  providers: ComputeProvider[];
  routes: ModelRoute[];
  kinds: string[];
  classifications: string[];
  /** False when no encryption key is set, so a credential cannot be stored. */
  credentialStorage: boolean;
};

export type ProviderCheck = {
  status: string;
  error?: string;
  models?: { id: string; family?: string }[];
};

export type Completion = {
  logicalModel: string;
  provider: string;
  model: string;
  content: string;
  finishReason: string;
  usage: TokenUsage;
  latencyMs: number;
};

export class ApiError extends Error {
  constructor(readonly status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }

  /** A missing or rejected credential, as opposed to a missing capability. */
  get unauthenticated(): boolean {
    return this.status === 401;
  }
}

const CREDENTIAL_KEY_ALT = 'ceap_api_key';

function getCookie(name: string): string {
  if (typeof document === 'undefined') return '';
  const value = `; ${document.cookie}`;
  const parts = value.split(`; ${name}=`);
  if (parts.length === 2) return decodeURIComponent(parts.pop()?.split(';').shift() ?? '');
  return '';
}

function setCookie(name: string, value: string, days = 365): void {
  if (typeof document === 'undefined') return;
  if (!value) {
    document.cookie = `${name}=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Lax`;
    return;
  }
  const date = new Date();
  date.setTime(date.getTime() + days * 24 * 60 * 60 * 1000);
  document.cookie = `${name}=${encodeURIComponent(value)}; path=/; expires=${date.toUTCString()}; SameSite=Lax`;
}

export function readCredential(): string {
  try {
    const val =
      localStorage.getItem(CREDENTIAL_KEY) ||
      localStorage.getItem(CREDENTIAL_KEY_ALT) ||
      sessionStorage.getItem(CREDENTIAL_KEY) ||
      getCookie(CREDENTIAL_KEY_ALT) ||
      '';
    if (val) {
      try {
        localStorage.setItem(CREDENTIAL_KEY, val);
        localStorage.setItem(CREDENTIAL_KEY_ALT, val);
        sessionStorage.setItem(CREDENTIAL_KEY, val);
        setCookie(CREDENTIAL_KEY_ALT, val);
      } catch {}
    }
    return val;
  } catch {
    return getCookie(CREDENTIAL_KEY_ALT);
  }
}

export function writeCredential(key: string): void {
  const trimmed = key.trim();
  try {
    if (trimmed === '') {
      localStorage.removeItem(CREDENTIAL_KEY);
      localStorage.removeItem(CREDENTIAL_KEY_ALT);
      sessionStorage.removeItem(CREDENTIAL_KEY);
      setCookie(CREDENTIAL_KEY_ALT, '');
      return;
    }
    localStorage.setItem(CREDENTIAL_KEY, trimmed);
    localStorage.setItem(CREDENTIAL_KEY_ALT, trimmed);
    sessionStorage.setItem(CREDENTIAL_KEY, trimmed);
    setCookie(CREDENTIAL_KEY_ALT, trimmed);
  } catch {}
}

function headers(): HeadersInit {
  const credential = readCredential();
  return credential === '' ? {} : { Authorization: `Bearer ${credential}` };
}

async function failure(response: Response): Promise<ApiError> {
  let message = response.statusText || `request failed with ${response.status}`;
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) message = body.error;
  } catch {
    // A non-JSON error body is not worth surfacing; the status carries the meaning.
  }
  return new ApiError(response.status, message);
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(`${BASE}${path}`, { headers: headers(), signal });
  if (!response.ok) throw await failure(response);
  return (await response.json()) as T;
}

/** Platform discovery is unauthenticated, so it works before a key is entered. */
export const fetchPlatform = (signal?: AbortSignal) => get<Platform>('/api/v1/platform', signal);
export const fetchIdentity = (signal?: AbortSignal) => get<Identity>('/api/v1/me', signal);
export const fetchModels = (signal?: AbortSignal) => get<ModelCatalogue>('/api/v1/models', signal);
export const fetchComputeHealth = (signal?: AbortSignal) =>
  get<ComputeHealth>('/api/v1/compute/health', signal);
export const fetchAssistants = (signal?: AbortSignal) =>
  get<{ assistants: Assistant[] }>('/api/v1/assistants', signal);
export const fetchConversations = (trash: boolean, signal?: AbortSignal) =>
  get<ConversationList>(`/api/v1/conversations${trash ? '?trash=true' : ''}`, signal);
export const fetchConversation = (id: string, signal?: AbortSignal) =>
  get<ConversationDetail>(`/api/v1/conversations/${encodeURIComponent(id)}`, signal);

export const fetchDocuments = (trash: boolean, signal?: AbortSignal) =>
  get<DocumentList>(`/api/v1/documents${trash ? '?trash=true' : ''}`, signal);
export const fetchFavorites = (signal?: AbortSignal) =>
  get<{ favorites: Favorite[] }>('/api/v1/favorites', signal);
export const fetchGraph = (term: string, signal?: AbortSignal) =>
  get<{ subgraph: Subgraph }>(`/api/v1/graph/neighbours?term=${encodeURIComponent(term)}`, signal);

async function send<T>(method: string, path: string, body?: unknown): Promise<T | undefined> {
  const response = await fetch(`${BASE}${path}`, {
    method,
    headers: body === undefined ? headers() : { ...headers(), 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!response.ok) throw await failure(response);
  // 204 is the normal answer for favourites and deletions; parsing it would throw.
  if (response.status === 204) return undefined;
  return (await response.json()) as T;
}

export async function deleteConversation(id: string): Promise<void> {
  await send('DELETE', `/api/v1/conversations/${encodeURIComponent(id)}`);
}

export async function restoreConversation(id: string): Promise<void> {
  await send('POST', `/api/v1/conversations/${encodeURIComponent(id)}/restore`);
}

export type UploadRequest = {
  title: string;
  content: string;
  classification: string;
  sourceSlug?: string;
  mimeType?: string;
};

/**
 * Uploads a document. The response is 202: the text is stored and queued, and
 * the embedding worker moves it to ready afterwards, so a client must not treat
 * a successful upload as a searchable document.
 */
export const uploadDocument = (request: UploadRequest) =>
  send<{ document: Document }>('POST', '/api/v1/documents', request);

export async function deleteDocument(id: string): Promise<void> {
  await send('DELETE', `/api/v1/documents/${encodeURIComponent(id)}`);
}

/** Restore answers 202: the row is back, but re-ingestion has not run yet. */
export const restoreDocument = (id: string) =>
  send<{ document: Document }>('POST', `/api/v1/documents/${encodeURIComponent(id)}/restore`);

/**
 * Permanently destroys a withdrawn document.
 *
 * The id is repeated as `confirm` because the gateway refuses a purge that does
 * not name what it is destroying — a second, deliberate step, never the first.
 */
export async function purgeDocument(id: string): Promise<void> {
  const encoded = encodeURIComponent(id);
  await send('DELETE', `/api/v1/documents/${encoded}/purge?confirm=${encoded}`);
}

export const searchKnowledge = (query: string, limit?: number) =>
  send<SearchResult>('POST', '/api/v1/knowledge/search', { query, ...(limit ? { limit } : {}) });

export const fetchProviderRegistry = (signal?: AbortSignal) =>
  get<ProviderRegistry>('/api/v1/admin/providers', signal);

export type ProviderInput = {
  slug: string;
  name: string;
  description?: string;
  kind: string;
  baseUrl: string;
  credential?: string;
  maxClassification: string;
  timeoutSeconds?: number;
};

export const createProvider = (input: ProviderInput) =>
  send<{ provider: ComputeProvider }>('POST', '/api/v1/admin/providers', input);

/**
 * Sends only the fields being changed.
 *
 * The credential is omitted rather than sent empty when it is not being
 * replaced: the portal is never given the stored value, so it has nothing to
 * send back, and an empty string would wipe it.
 */
export const updateProvider = (slug: string, patch: Partial<ProviderInput> & { enabled?: boolean }) =>
  send<{ provider: ComputeProvider }>('PATCH', `/api/v1/admin/providers/${encodeURIComponent(slug)}`, patch);

export async function deleteProvider(slug: string): Promise<void> {
  await send('DELETE', `/api/v1/admin/providers/${encodeURIComponent(slug)}`);
}

export const checkProvider = (slug: string) =>
  send<ProviderCheck>('POST', `/api/v1/admin/providers/${encodeURIComponent(slug)}/check`);

export const saveRoute = (route: { logical: string; provider: string; model: string; default?: boolean; enabled?: boolean }) =>
  send<{ route: ModelRoute }>('PUT', '/api/v1/admin/routes', route);

export async function deleteRoute(logical: string): Promise<void> {
  await send('DELETE', `/api/v1/admin/routes/${encodeURIComponent(logical)}`);
}

export async function setFavorite(
  kind: FavoriteKind,
  targetId: string,
  marked: boolean,
): Promise<void> {
  await send(marked ? 'PUT' : 'DELETE', '/api/v1/favorites', { kind, targetId });
}

export type PlatformSetting = {
  key: string;
  value: string;
  description: string;
  updatedAt: string;
  updatedBy: string;
};

export const fetchPlatformSettings = (signal?: AbortSignal) =>
  get<PlatformSetting[]>('/api/v1/admin/settings', signal);

export const updatePlatformSetting = (key: string, value: any, description?: string) =>
  send<PlatformSetting>('PUT', `/api/v1/admin/settings/${encodeURIComponent(key)}`, { value, description });

export type PromptTemplate = {
  id: string;
  slug: string;
  name: string;
  description: string;
  template: string;
  variables: string[];
  createdBy: string;
  createdAt: string;
  updatedAt: string;
};

export const fetchPromptTemplates = (signal?: AbortSignal) =>
  get<PromptTemplate[]>('/api/v1/prompts', signal);

export const createPromptTemplate = (input: {
  slug: string;
  name: string;
  description?: string;
  template: string;
  variables?: string[];
}) => send<PromptTemplate>('POST', '/api/v1/admin/prompts', input);

export async function deletePromptTemplate(id: string): Promise<void> {
  await send('DELETE', `/api/v1/admin/prompts/${encodeURIComponent(id)}`);
}

export type ChatMessage = { role: 'system' | 'user' | 'assistant'; content: string };

/**
 * Either form is accepted, never both: `messages` for a stateless call where the
 * caller holds the transcript, or `message` with an assistant or conversation
 * for one the platform stores.
 */
export type ChatRequest = {
  model?: string;
  messages?: ChatMessage[];
  assistant?: string;
  conversationId?: string;
  message?: string;
  temperature?: number;
  maxTokens?: number;
};

export type StreamHandlers = {
  onContent: (delta: string) => void;
  onDone: (finishReason: string, usage: TokenUsage | undefined) => void;
  /**
   * Reports the stored conversation before any content, so a newly created one
   * can be tracked even if the stream later fails.
   */
  onConversation?: (id: string, assistant: string) => void;
};

type StreamFrame = {
  content?: string;
  done?: boolean;
  finishReason?: string;
  usage?: TokenUsage;
  conversationId?: string;
  assistant?: string;
};

/**
 * Streams a completion over Server-Sent Events.
 *
 * EventSource cannot send an Authorization header, so this reads the body with
 * fetch instead. Frames are reassembled across network chunks because a chunk
 * boundary can land anywhere, including inside a single event.
 */
export async function streamChat(
  request: ChatRequest,
  handlers: StreamHandlers,
  signal?: AbortSignal,
): Promise<void> {
  const response = await fetch(`${BASE}/api/v1/chat/completions`, {
    method: 'POST',
    headers: { ...headers(), 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...request, stream: true }),
    signal,
  });
  if (!response.ok) throw await failure(response);
  if (!response.body) throw new ApiError(response.status, 'the response carried no body to stream');

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';

  const consume = (frame: string): void => {
    let event = '';
    const data: string[] = [];
    for (const line of frame.split('\n')) {
      if (line.startsWith('event: ')) event = line.slice(7).trim();
      else if (line.startsWith('data: ')) data.push(line.slice(6));
    }
    if (data.length === 0) return;

    const payload = data.join('\n');
    if (payload === '[DONE]') return;
    if (event === 'error') throw new ApiError(502, 'the stream was interrupted');

    const chunk = JSON.parse(payload) as StreamFrame;
    if (chunk.conversationId) handlers.onConversation?.(chunk.conversationId, chunk.assistant ?? '');
    if (chunk.content) handlers.onContent(chunk.content);
    if (chunk.done) handlers.onDone(chunk.finishReason ?? '', chunk.usage);
  };

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });

    let boundary = buffer.indexOf('\n\n');
    while (boundary !== -1) {
      consume(buffer.slice(0, boundary));
      buffer = buffer.slice(boundary + 2);
      boundary = buffer.indexOf('\n\n');
    }
  }
}
