import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useCallback, useSyncExternalStore } from 'react';
import {
  ApiError,
  fetchComputeHealth,
  fetchIdentity,
  fetchModels,
  fetchPlatform,
  readCredential,
  writeCredential,
  type ComputeHealth,
  type Identity,
  type ModelCatalogue,
  type Platform,
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
