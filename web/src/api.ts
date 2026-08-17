// Typed client for the AI Gateway.
//
// The portal talks to the Control Plane and nothing else; it never addresses a
// model provider directly (ARCHITECTURE-v1 section 2).

const BASE = (import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080').replace(/\/+$/, '');

// The credential lives in sessionStorage so it dies with the tab. This is the
// development bridge until the Identity Service issues JWTs to the portal; a
// key pasted here should carry only the scopes the operator actually needs.
const CREDENTIAL_KEY = 'ceap.apiKey';

export type Identity = {
  keyId: string;
  name: string;
  scopes: string[];
  companyId?: string;
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

export function readCredential(): string {
  return sessionStorage.getItem(CREDENTIAL_KEY) ?? '';
}

export function writeCredential(key: string): void {
  const trimmed = key.trim();
  if (trimmed === '') {
    sessionStorage.removeItem(CREDENTIAL_KEY);
    return;
  }
  sessionStorage.setItem(CREDENTIAL_KEY, trimmed);
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

export type ChatMessage = { role: 'system' | 'user' | 'assistant'; content: string };

export type ChatRequest = {
  model?: string;
  messages: ChatMessage[];
  temperature?: number;
  maxTokens?: number;
};

export type StreamHandlers = {
  onContent: (delta: string) => void;
  onDone: (finishReason: string, usage: TokenUsage | undefined) => void;
};

type StreamFrame = {
  content?: string;
  done?: boolean;
  finishReason?: string;
  usage?: TokenUsage;
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
