import type { TokenUsageSummary } from "@/lib/types";
import { compactNumber, formatCost, formatLatency } from "../utils";

export function ProviderAnalytics({ providerUsage }: { providerUsage: TokenUsageSummary[] }) {
  return (
    <section className="mb-6 overflow-hidden rounded-lg border border-stroke bg-panel">
      <div className="border-b border-stroke p-5">
        <h3 className="font-mono font-semibold">Provider & Model Breakdown</h3>
        <p className="text-sm text-content-muted">Requests, tokens, cost, and latency grouped by provider, model, and level.</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
            <tr>
              <th className="px-4 py-3">Provider</th>
              <th className="px-4 py-3">Model</th>
              <th className="px-4 py-3">Level</th>
              <th className="px-4 py-3">Requests</th>
              <th className="px-4 py-3">Tokens</th>
              <th className="px-4 py-3">Avg Latency</th>
              <th className="px-4 py-3">Cost</th>
            </tr>
          </thead>
          <tbody>
            {providerUsage.map((item) => (
              <tr key={`${item.provider}:${item.model}:${item.level_group}`} className="border-b border-stroke/60 transition hover:bg-slate-900/50">
                <td className="px-4 py-3 font-medium capitalize">{item.provider}</td>
                <td className="px-4 py-3 font-mono text-xs text-content-muted">{item.model}</td>
                <td className="px-4 py-3 text-content-muted">{item.level_group || "unknown"}</td>
                <td className="px-4 py-3 font-mono">{compactNumber(item.requests)}</td>
                <td className="px-4 py-3 font-mono">{compactNumber(item.total_tokens)}</td>
                <td className="px-4 py-3 font-mono">{formatLatency(item.avg_latency_ms)}</td>
                <td className="px-4 py-3 font-mono font-semibold">{formatCost(item.cost_usd)}</td>
              </tr>
            ))}
            {providerUsage.length === 0 && (
              <tr>
                <td className="px-4 py-8 text-center text-content-muted" colSpan={7}>
                  No provider usage recorded.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
