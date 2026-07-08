import { PROVIDERS } from "@/lib/model-options";

export function ProviderSummarySkeleton() {
  return (
    <div className="grid gap-3 md:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="rounded-lg border border-stroke bg-card p-4 shadow-sm animate-pulse">
          <div className="flex items-start justify-between gap-3">
            <div className="w-full space-y-2">
              <div className="h-3 w-32 rounded bg-surface-muted" />
              <div className="h-7 w-20 rounded bg-surface-muted" />
              <div className="h-3 w-44 rounded bg-surface-muted" />
            </div>
            <div className="size-9 shrink-0 rounded-lg bg-surface-muted" />
          </div>
        </div>
      ))}
    </div>
  );
}

export function ProviderTableSkeleton() {
  return (
    <div className="glass-panel overflow-hidden rounded-lg animate-fade-in">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[980px] text-left text-sm">
          <thead className="border-b border-stroke bg-surface/70 text-[11px] uppercase tracking-wide text-content-muted">
            <tr>
              <th className="w-[30%] px-4 py-3">Provider</th>
              <th className="w-[12%] px-4 py-3">Status</th>
              <th className="w-[24%] px-4 py-3">Models</th>
              <th className="w-[34%] px-4 py-3">Credentials</th>
            </tr>
          </thead>
          <tbody>
            {PROVIDERS.map((provider) => (
              <tr key={provider} className="border-t border-stroke animate-pulse">
                <td className="px-4 py-4">
                  <div className="flex items-start gap-3">
                    <div className="size-10 shrink-0 rounded-lg bg-surface-muted" />
                    <div className="min-w-0 flex-1 space-y-2">
                      <div className="h-5 w-28 rounded bg-surface-muted" />
                      <div className="h-3 w-full max-w-[300px] rounded bg-surface-muted" />
                    </div>
                  </div>
                </td>
                <td className="px-4 py-4">
                  <div className="h-6 w-16 rounded-full bg-surface-muted" />
                </td>
                <td className="px-4 py-4">
                  <div className="flex flex-wrap gap-1.5">
                    <div className="h-5 w-24 rounded-full bg-surface-muted" />
                    <div className="h-5 w-28 rounded-full bg-surface-muted" />
                    <div className="h-5 w-20 rounded-full bg-surface-muted" />
                  </div>
                </td>
                <td className="px-4 py-4">
                  <div className="h-16 rounded-md bg-surface-muted" />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
