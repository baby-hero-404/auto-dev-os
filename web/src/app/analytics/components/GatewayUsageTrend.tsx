import {
  AreaChart,
  Area,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { compactNumber, formatCost, formatLatency } from "../utils";

export function GatewayUsageTrend({ gatewayUsage }: { gatewayUsage: any[] }) {
  return (
    <section className="mb-6 rounded-lg border border-stroke bg-panel p-5">
      <h3 className="mb-1 font-mono font-semibold">Gateway Usage Trend</h3>
      <p className="mb-4 text-sm text-content-muted">Daily requests, token consumption, spend, and latency for the selected scope.</p>
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={gatewayUsage}>
            <defs>
              <linearGradient id="gradGatewayRequests" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#38bdf8" stopOpacity={0.32} />
                <stop offset="95%" stopColor="#38bdf8" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="gradGatewayTokens" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="#a78bfa" stopOpacity={0.28} />
                <stop offset="95%" stopColor="#a78bfa" stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid stroke="rgba(148, 163, 184, 0.1)" vertical={false} />
            <XAxis
              dataKey="bucket"
              stroke="#94a3b8"
              fontSize={11}
              tickFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "short", day: "numeric" })}
            />
            <YAxis yAxisId="requests" stroke="#94a3b8" fontSize={11} allowDecimals={false} />
            <YAxis yAxisId="tokens" orientation="right" stroke="#94a3b8" fontSize={11} tickFormatter={compactNumber} />
            <Tooltip
              contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
              labelFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "long", day: "numeric" })}
              formatter={(value: any, name: any) => {
                if (name === "cost_usd") return [formatCost(Number(value)), "Cost"];
                if (name === "avg_latency_ms") return [formatLatency(Number(value)), "Avg Latency"];
                return [compactNumber(Number(value)), name === "requests" ? "Requests" : "Tokens"];
              }}
            />
            <Area yAxisId="requests" type="monotone" dataKey="requests" stroke="#38bdf8" fill="url(#gradGatewayRequests)" />
            <Area yAxisId="tokens" type="monotone" dataKey="total_tokens" stroke="#a78bfa" fill="url(#gradGatewayTokens)" />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}
