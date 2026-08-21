import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useCallback, useSyncExternalStore } from 'react';
import {
  ApiError,
  fetchAssistants,
  fetchComputeHealth,
  fetchConversation,
  fetchConversations,
  fetchDocuments,
  fetchFavorites,
  fetchIdentity,
  fetchApiKeys,
  fetchModels,
  fetchPlatform,
  fetchPlatformSettings,
  fetchPromptTemplates,
  fetchProviderRegistry,
  readCredential,
  setFavorite,
  writeCredential,
  type ApiKeyRecord,
  type Assistant,
  type ComputeHealth,
  type ConversationDetail,
  type ConversationList,
  type DocumentList,
  type Favorite,
  type FavoriteKind,
  type Identity,
  type ModelCatalogue,
  type Platform,
  type PlatformSetting,
  type PromptTemplate,
  type ProviderRegistry,
} from './api';

// React Query defaults its error type to `unknown`. Its Register interface is
// declared inside query-core without being exported, so module augmentation
// does not merge with it; the error type is named at each call site instead.
// Everything here rejects with an Error, and with ApiError for anything the
// gateway actually answered.

// The credential is module state rather than React state because api.ts reads
// it directly on every request; useSyncExternalStore keeps the UI in step
// without making every caller thread it through props.
const listeners = new Set<() => void>();

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function useCredential(): [string, (key: string) => void] {
  const queryClient = useQueryClient();
  const credential = useSyncExternalStore(subscribe, readCredential, () => '');

  const set = useCallback(
    (key: string) => {
      writeCredential(key);
      listeners.forEach((listener) => listener());
      // Everything authenticated was fetched as somebody else; none of it is
      // valid for the new caller.
      void queryClient.invalidateQueries();
    },
    [queryClient],
  );

  return [credential, set];
}

/** Retrying a rejected credential or a missing scope only wastes quota. */
function retryUnlessClientError(failureCount: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return failureCount < 2;
}

export function usePlatform(): UseQueryResult<Platform, Error> {
  return useQuery<Platform, Error>({
    queryKey: ['platform'],
    queryFn: ({ signal }) => fetchPlatform(signal),
    retry: retryUnlessClientError,
  });
}

export function useIdentity(): UseQueryResult<Identity, Error> {
  const [credential] = useCredential();
  return useQuery<Identity, Error>({
    queryKey: ['identity', credential],
    queryFn: ({ signal }) => fetchIdentity(signal),
    enabled: credential !== '',
    retry: retryUnlessClientError,
  });
}

export function useModels(enabled: boolean): UseQueryResult<ModelCatalogue, Error> {
  const [credential] = useCredential();
  return useQuery<ModelCatalogue, Error>({
    queryKey: ['models', credential],
    queryFn: ({ signal }) => fetchModels(signal),
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

export function useComputeHealth(enabled: boolean): UseQueryResult<ComputeHealth, Error> {
  const [credential] = useCredential();
  return useQuery<ComputeHealth, Error>({
    queryKey: ['compute-health', credential],
    queryFn: ({ signal }) => fetchComputeHealth(signal),
    enabled: enabled && credential !== '',
    // The compute plane can come and go independently of this page.
    refetchInterval: 30_000,
    retry: retryUnlessClientError,
  });
}

export function useAssistants(enabled: boolean): UseQueryResult<Assistant[], Error> {
  const [credential] = useCredential();
  return useQuery<Assistant[], Error>({
    queryKey: ['assistants', credential],
    queryFn: async ({ signal }) => (await fetchAssistants(signal)).assistants ?? [],
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

export function useConversations(enabled: boolean, trash = false): UseQueryResult<ConversationList, Error> {
  const [credential] = useCredential();
  return useQuery<ConversationList, Error>({
    queryKey: ['conversations', credential, trash],
    queryFn: ({ signal }) => fetchConversations(trash, signal),
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

export function useConversation(id: string | null): UseQueryResult<ConversationDetail, Error> {
  const [credential] = useCredential();
  return useQuery<ConversationDetail, Error>({
    queryKey: ['conversation', credential, id],
    queryFn: ({ signal }) => fetchConversation(id as string, signal),
    enabled: credential !== '' && id !== null,
    retry: retryUnlessClientError,
  });
}

export function useDocuments(enabled: boolean, trash = false): UseQueryResult<DocumentList, Error> {
  const [credential] = useCredential();
  return useQuery<DocumentList, Error>({
    queryKey: ['documents', credential, trash],
    queryFn: ({ signal }) => fetchDocuments(trash, signal),
    enabled: enabled && credential !== '',
    // Ingestion is asynchronous, so a document uploaded a moment ago is still
    // moving from pending to ready while the page is open.
    refetchInterval: (query) =>
      (query.state.data?.documents ?? []).some(
        (document) => document.status === 'pending' || document.status === 'processing',
      )
        ? 4_000
        : false,
    retry: retryUnlessClientError,
  });
}

export function useFavorites(enabled: boolean): UseQueryResult<Favorite[], Error> {
  const [credential] = useCredential();
  return useQuery<Favorite[], Error>({
    queryKey: ['favorites', credential],
    queryFn: async ({ signal }) => (await fetchFavorites(signal)).favorites ?? [],
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

/**
 * Marks or unmarks a favourite and refreshes the list.
 *
 * The call is not optimistic: the server drops a mark whose target it cannot
 * resolve, so showing one before it answers could show a star that disappears
 * on the next load.
 */
export function useToggleFavorite(): (
  kind: FavoriteKind,
  targetId: string,
  marked: boolean,
) => Promise<void> {
  const queryClient = useQueryClient();
  const [credential] = useCredential();
  return useCallback(
    async (kind, targetId, marked) => {
      await setFavorite(kind, targetId, marked);
      // Immediately invalidate and refetch favorites to sync state across the app
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['favorites'] }),
        queryClient.refetchQueries({ queryKey: ['favorites'] }),
        queryClient.refetchQueries({ queryKey: ['favorites', credential] }),
      ]);
    },
    [queryClient, credential],
  );
}

export function useProviderRegistry(enabled: boolean): UseQueryResult<ProviderRegistry, Error> {
  const [credential] = useCredential();
  return useQuery<ProviderRegistry, Error>({
    queryKey: ['provider-registry', credential],
    queryFn: ({ signal }) => fetchProviderRegistry(signal),
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

/** Refreshes the routing table after a provider or route changes. */
export function useRefreshProviders(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['provider-registry'] });
    // The model catalogue and compute health are both derived from the routing
    // table, so a change here makes both of them stale.
    void queryClient.invalidateQueries({ queryKey: ['models'] });
    void queryClient.invalidateQueries({ queryKey: ['compute-health'] });
  }, [queryClient]);
}

/** Refreshes the history list after a turn changes a title or its counters. */
export function useRefreshHistory(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['conversations'] });
  }, [queryClient]);
}

/** Refreshes the corpus after an upload or a deletion. */
export function useRefreshDocuments(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['documents'] });
  }, [queryClient]);
}

export function usePlatformSettings(enabled: boolean): UseQueryResult<PlatformSetting[], Error> {
  const [credential] = useCredential();
  return useQuery<PlatformSetting[], Error>({
    queryKey: ['platform-settings', credential],
    queryFn: ({ signal }) => fetchPlatformSettings(signal),
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

export function usePromptTemplates(enabled: boolean): UseQueryResult<PromptTemplate[], Error> {
  const [credential] = useCredential();
  return useQuery<PromptTemplate[], Error>({
    queryKey: ['prompt-templates', credential],
    queryFn: ({ signal }) => fetchPromptTemplates(signal),
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

export function useRefreshSettings(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['platform-settings'] });
  }, [queryClient]);
}

export function useRefreshPrompts(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['prompt-templates'] });
  }, [queryClient]);
}

export function useApiKeys(enabled: boolean): UseQueryResult<ApiKeyRecord[], Error> {
  const [credential] = useCredential();
  return useQuery<ApiKeyRecord[], Error>({
    queryKey: ['admin-api-keys', credential],
    queryFn: ({ signal }) => fetchApiKeys(signal),
    enabled: enabled && credential !== '',
    retry: retryUnlessClientError,
  });
}

export function useRefreshApiKeys(): () => void {
  const queryClient = useQueryClient();
  return useCallback(() => {
    void queryClient.invalidateQueries({ queryKey: ['admin-api-keys'] });
  }, [queryClient]);
}

/**
 * Scope checks drive what the UI offers. This is convenience only: the backend
 * authorizes every action regardless of what the portal chose to show
 * (ARCHITECTURE-v1 section 5).
 */
export function useScopes(): { scopes: string[]; has: (scope: string) => boolean } {
  const identity = useIdentity();
  const scopes = identity.data?.scopes ?? [];
  return { scopes, has: (scope: string) => scopes.includes(scope) };
}

