import {
  BarChart,
  Bar,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { compactNumber, formatCost } from "../utils";

export interface KeyLabelUsage {
  key_label: string;
  success: number;
  failed: number;
  total_tokens: number;
  cost_usd: number;
}

export function VirtualKeyAnalytics({ keyLabelUsage }: { keyLabelUsage: KeyLabelUsage[] }) {
  return (
    <div className="mb-6 grid gap-5 lg:grid-cols-5">
      {/* Virtual Key Report */}
      <section className="lg:col-span-3 overflow-hidden rounded-lg border border-stroke bg-panel flex flex-col">
        <div className="border-b border-stroke p-5">
          <h3 className="font-mono font-semibold">Virtual Key Usage & Spend</h3>
          <p className="text-sm text-content-muted">Spend, token counts, and API call volumes by key label.</p>
        </div>
        <div className="overflow-x-auto flex-1">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
              <tr>
                <th className="px-4 py-3">Key Label</th>
                <th className="px-4 py-3">Requests</th>
                <th className="px-4 py-3">Tokens</th>
                <th className="px-4 py-3">Cost</th>
              </tr>
            </thead>
            <tbody>
              {keyLabelUsage.map((key) => (
                <tr key={key.key_label} className="border-b border-stroke/60 transition hover:bg-slate-900/50">
                  <td className="px-4 py-3">
                    <div className="font-medium font-mono text-xs flex items-center gap-1.5 text-white">
                      <span className="inline-block w-1.5 h-1.5 rounded-full bg-brand-primary" />
                      {key.key_label}
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-content-muted">
                    {compactNumber(key.success + key.failed)}
                    <span className="ml-1.5 text-[10px] text-emerald-400 font-semibold">
                      ({key.success + key.failed > 0 ? Math.round((key.success / (key.success + key.failed)) * 100) : 0}% success)
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-content-muted">{compactNumber(key.total_tokens)}</td>
                  <td className="px-4 py-3 font-mono text-xs text-white font-semibold">{formatCost(key.cost_usd)}</td>
                </tr>
              ))}
              {keyLabelUsage.length === 0 && (
                <tr>
                  <td className="px-4 py-8 text-center text-content-muted" colSpan={4}>
                    No virtual key usage recorded.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Success vs Failure Stacked Bar Chart */}
      <section className="lg:col-span-2 rounded-lg border border-stroke bg-panel p-5 flex flex-col">
        <h3 className="mb-1 font-mono font-semibold">API Request Success Rate</h3>
        <p className="mb-4 text-sm text-content-muted">Successful vs failed API calls by Key Label.</p>
        <div className="h-64 flex-1">
          {keyLabelUsage.length === 0 ? (
            <div className="h-full flex items-center justify-center text-sm text-content-muted">
              No chart data available.
            </div>
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={keyLabelUsage} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
                <CartesianGrid stroke="rgba(148, 163, 184, 0.1)" vertical={false} />
                <XAxis dataKey="key_label" stroke="#94a3b8" fontSize={11} tickFormatter={(v) => v.slice(0, 10) + (v.length > 10 ? "…" : "")} />
                <YAxis stroke="#94a3b8" fontSize={11} allowDecimals={false} />
                <Tooltip
                  contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
                />
                <Legend verticalAlign="top" height={36} iconType="circle" wrapperStyle={{ fontSize: 11, color: "#94a3b8" }} />
                <Bar dataKey="success" name="Success" stackId="a" fill="#22c55e" radius={[0, 0, 0, 0]} />
                <Bar dataKey="failed" name="Failed" stackId="a" fill="#ef4444" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </section>
    </div>
  );
}
