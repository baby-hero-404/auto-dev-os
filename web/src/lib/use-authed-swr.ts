"use client";

import useSWR, { type SWRConfiguration } from "swr";
import { useSession } from "@/lib/session";

/**
 * A thin wrapper around `useSWR` that:
 *
 * 1. Automatically injects the current session token into the fetcher
 *    WITHOUT including it in the cache key.
 * 2. Returns `null` key (disabling fetch) when no session exists.
 *
 * This decouples cache identity from the auth credential, preventing
 * all SWR hooks from invalidating simultaneously when the token string
 * reference changes (e.g., session hydration from localStorage).
 *
 * Usage:
 *   const { data } = useAuthedSWR(
 *     ["projects", orgID],
 *     (token) => api.listProjects(orgID, token),
 *   );
 */
export function useAuthedSWR<T>(
  key: readonly unknown[] | null,
  fetcher: (token: string) => Promise<T>,
  options?: SWRConfiguration<T>,
) {
  const session = useSession();
  const token = session?.token ?? null;

  return useSWR<T>(
    token && key ? key : null,
    () => fetcher(token!),
    options,
  );
}
