"use client";

import { SWRConfig } from "swr";

/**
 * Global SWR configuration provider.
 *
 * Key decisions:
 * - `dedupingInterval: 5000` — prevents the same key from being fetched
 *   more than once within a 5-second window (guards against layout
 *   re-renders triggering duplicate requests).
 * - `revalidateOnFocus: false` — disables the default SWR behavior of
 *   refetching every hook when the browser tab regains focus. For a
 *   dashboard with 10+ SWR hooks per page, this prevents a thundering
 *   herd of API calls each time a developer alt-tabs back.
 * - `revalidateIfStale: true` — stale data is still served instantly
 *   from cache while a background revalidation happens.
 */
export function SWRProvider({ children }: { children: React.ReactNode }) {
  return (
    <SWRConfig
      value={{
        dedupingInterval: 5000,
        revalidateOnFocus: false,
        revalidateIfStale: true,
        errorRetryCount: 2,
      }}
    >
      {children}
    </SWRConfig>
  );
}
