"use client";

import { BarChart, Bar, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Activity, Coins, Gauge, Router, type LucideIcon } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { TokenUsageSummary } from "@/lib/types";

function formatCost(value: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 4 }).format(value);
}

function compactNumber(value: number) {
  return new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value);
}

export default function GatewayPage() {
  const session = useSession();
  const { data: usage } = useAuthedSWR(
    ["token-usage"],
    (token) => api.tokenUsage(token, 30),
  );

  const safeUsage = usage || [];

  const totals = safeUsage.reduce(
    (acc, item) => ({
      requests: acc.requests + item.requests,
      tokens: acc.tokens + item.total_tokens,
      cost: acc.cost + item.cost_usd,
      latencyWeighted: acc.latencyWeighted + item.avg_latency_ms * item.requests,
    }),
    { requests: 0, tokens: 0, cost: 0, latencyWeighted: 0 },
  );
  const avgLatency = totals.requests > 0 ? totals.latencyWeighted / totals.requests : 0;
  const chartData = safeUsage.map((item: TokenUsageSummary) => ({
    name: `${item.provider}/${item.tier}`,
    tokens: item.total_tokens,
    cost: item.cost_usd,
  }));

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Gateway</h2>
          <p className="mt-1 text-sm text-content-muted">
            Model routing, token spend, and latency over the last 30 days.
          </p>
        </div>
        <div className="rounded-full border border-stroke bg-panel px-3 py-1 text-xs text-content-muted">
          Phase 4 telemetry
        </div>
      </div>

      <div className="mb-5 grid gap-4 md:grid-cols-4">
        <MetricCard icon={Router} label="Requests" value={compactNumber(totals.requests)} />
        <MetricCard icon={Activity} label="Tokens" value={compactNumber(totals.tokens)} />
        <MetricCard icon={Coins} label="Cost" value={formatCost(totals.cost)} />
        <MetricCard icon={Gauge} label="Avg latency" value={`${Math.round(avgLatency)} ms`} />
      </div>

      <section className="mb-5 rounded-lg border border-stroke bg-panel p-5">
        <div className="mb-4">
          <h3 className="font-mono font-semibold">Token Usage By Route</h3>
          <p className="text-sm text-content-muted">Grouped by provider and routing tier.</p>
        </div>
        <div className="h-72">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={chartData}>
              <CartesianGrid stroke="rgba(148, 163, 184, 0.14)" vertical={false} />
              <XAxis dataKey="name" stroke="#94a3b8" fontSize={12} />
              <YAxis stroke="#94a3b8" fontSize={12} tickFormatter={compactNumber} />
              <Tooltip
                contentStyle={{ background: "#0f172a", border: "1px solid rgba(148, 163, 184, 0.22)" }}
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                formatter={(value: any, name: any) => [name === "cost" ? formatCost(Number(value)) : compactNumber(Number(value)), name]}
              />
              <Bar dataKey="tokens" fill="var(--color-brand-primary)" radius={[6, 6, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </section>

      <div className="overflow-hidden rounded-lg border border-stroke bg-panel">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
            <tr>
              <th className="px-4 py-3">Route</th>
              <th className="px-4 py-3">Model</th>
              <th className="px-4 py-3">Requests</th>
              <th className="px-4 py-3">Tokens</th>
              <th className="px-4 py-3">Cost</th>
              <th className="px-4 py-3">Latency</th>
            </tr>
          </thead>
          <tbody>
            {safeUsage.map((item) => (
              <tr key={`${item.provider}-${item.model}-${item.tier}`} className="border-b border-stroke/60">
                <td className="px-4 py-3 font-mono text-brand-primary">{item.provider}/{item.tier}</td>
                <td className="px-4 py-3">{item.model}</td>
                <td className="px-4 py-3">{compactNumber(item.requests)}</td>
                <td className="px-4 py-3">{compactNumber(item.total_tokens)}</td>
                <td className="px-4 py-3">{formatCost(item.cost_usd)}</td>
                <td className="px-4 py-3">{Math.round(item.avg_latency_ms)} ms</td>
              </tr>
            ))}
            {safeUsage.length === 0 && (
              <tr>
                <td className="px-4 py-8 text-center text-content-muted" colSpan={6}>
                  No gateway usage recorded yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </DashboardLayout>
  );
}

function MetricCard({ icon: Icon, label, value }: { icon: LucideIcon; label: string; value: string }) {
  return (
    <article className="rounded-lg border border-stroke bg-panel p-4">
      <div className="mb-3 grid size-9 place-items-center rounded-md bg-brand-primary/10 text-brand-primary">
        <Icon size={18} />
      </div>
      <div className="font-mono text-xl font-semibold">{value}</div>
      <div className="text-sm text-content-muted">{label}</div>
    </article>
  );
}
