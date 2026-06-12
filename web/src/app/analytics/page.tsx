"use client";

import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  CartesianGrid,
  Cell,
  PieChart,
  Pie,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Activity,
  Bot,
  CheckCircle2,
  Coins,
  FolderKanban,
  GitPullRequest,
  type LucideIcon,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";

function compactNumber(value: number) {
  return new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value);
}
function formatCost(value: number) {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(value);
}
function formatDuration(ms: number) {
  if (ms < 60_000) return `${Math.round(ms / 1000)}s`;
  return `${Math.round(ms / 60_000)}m`;
}

const STATUS_COLORS: Record<string, string> = {
  todo: "#64748b",
  analyzing: "#f59e0b",
  spec_review: "#a78bfa",
  assigned: "#38bdf8",
  planning: "#818cf8",
  coding: "#22c55e",
  reviewing: "#06b6d4",
  fixing: "#fb923c",
  testing: "#14b8a6",
  human_review: "#e879f9",
  merged: "#34d399",
  in_progress: "#60a5fa",
  failed: "#ef4444",
  completed: "#22c55e",
};

export default function AnalyticsPage() {
  const session = useSession();
  const orgID = session?.user.org_id ?? "";

  const { data: overview } = useAuthedSWR(
    orgID ? ["analytics-overview", orgID] : null,
    (token) => api.analyticsOverview(token, orgID),
  );
  const { data: agentStats = [] } = useAuthedSWR(
    orgID ? ["analytics-agents", orgID] : null,
    (token) => api.analyticsAgents(token, orgID),
  );
  const { data: taskAnalytics } = useAuthedSWR(
    orgID ? ["analytics-tasks", orgID] : null,
    (token) => api.analyticsTasks(token, orgID),
  );
  const { data: workflowAnalytics } = useAuthedSWR(
    orgID ? ["analytics-workflows", orgID] : null,
    (token) => api.analyticsWorkflows(token, orgID),
  );

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="font-mono text-2xl font-semibold">Analytics</h1>
          <p className="mt-1 text-sm text-content-muted">
            Platform performance, agent metrics, and workflow health.
          </p>
        </div>
        <div className="rounded-full border border-stroke bg-panel px-3 py-1 text-xs text-content-muted">
          Phase 5 dashboard
        </div>
      </div>

      {/* Overview stat cards */}
      <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6">
        <StatCard icon={FolderKanban} label="Projects" value={compactNumber(overview?.total_projects ?? 0)} />
        <StatCard icon={Activity} label="Active Tasks" value={compactNumber(overview?.active_tasks ?? 0)} accent />
        <StatCard icon={CheckCircle2} label="Success Rate" value={`${Math.round(overview?.success_rate ?? 0)}%`} />
        <StatCard icon={Bot} label="Running Agents" value={`${overview?.running_agents ?? 0} / ${overview?.total_agents ?? 0}`} />
        <StatCard icon={GitPullRequest} label="Open PRs" value={compactNumber(overview?.open_prs ?? 0)} />
        <StatCard icon={Coins} label="Token Cost" value={formatCost(overview?.total_token_cost ?? 0)} />
      </div>

      {/* Charts row */}
      <div className="mb-6 grid gap-5 lg:grid-cols-2">
        {/* Task Throughput Chart */}
        <section className="rounded-lg border border-stroke bg-panel p-5">
          <h3 className="mb-1 font-mono font-semibold">Task Throughput</h3>
          <p className="mb-4 text-sm text-content-muted">Tasks created, completed, and failed over time.</p>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={taskAnalytics?.time_series ?? []}>
                <defs>
                  <linearGradient id="gradCreated" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#60a5fa" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#60a5fa" stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id="gradCompleted" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#22c55e" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#22c55e" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid stroke="rgba(148, 163, 184, 0.1)" vertical={false} />
                <XAxis
                  dataKey="bucket"
                  stroke="#94a3b8"
                  fontSize={11}
                  tickFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "short", day: "numeric" })}
                />
                <YAxis stroke="#94a3b8" fontSize={11} allowDecimals={false} />
                <Tooltip
                  contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
                  labelFormatter={(v) => new Date(v).toLocaleDateString("en", { month: "long", day: "numeric" })}
                />
                <Area type="monotone" dataKey="created" stroke="#60a5fa" fill="url(#gradCreated)" />
                <Area type="monotone" dataKey="completed" stroke="#22c55e" fill="url(#gradCompleted)" />
                <Area type="monotone" dataKey="failed" stroke="#ef4444" fillOpacity={0} />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </section>

        {/* Status Distribution Chart */}
        <section className="rounded-lg border border-stroke bg-panel p-5">
          <h3 className="mb-1 font-mono font-semibold">Task Status Distribution</h3>
          <p className="mb-4 text-sm text-content-muted">Current task breakdown by lifecycle state.</p>
          <div className="flex items-center gap-6">
            <div className="h-52 w-52 shrink-0">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie
                    data={taskAnalytics?.distribution ?? []}
                    dataKey="count"
                    nameKey="status"
                    cx="50%"
                    cy="50%"
                    innerRadius={45}
                    outerRadius={80}
                    paddingAngle={2}
                    strokeWidth={0}
                  >
                    {(taskAnalytics?.distribution ?? []).map((entry) => (
                      <Cell key={entry.status} fill={STATUS_COLORS[entry.status] ?? "#475569"} />
                    ))}
                  </Pie>
                  <Tooltip
                    contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
                  />
                </PieChart>
              </ResponsiveContainer>
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-2 text-xs">
              {(taskAnalytics?.distribution ?? []).map((d) => (
                <div key={d.status} className="flex items-center gap-2">
                  <span className="block size-2.5 rounded-full" style={{ background: STATUS_COLORS[d.status] ?? "#475569" }} />
                  <span className="text-content-muted">{d.status}</span>
                  <span className="font-mono font-semibold">{d.count}</span>
                </div>
              ))}
            </div>
          </div>
        </section>
      </div>

      {/* Workflow Analytics */}
      <section className="mb-6 rounded-lg border border-stroke bg-panel p-5">
        <div className="mb-4 flex items-center justify-between">
          <div>
            <h3 className="font-mono font-semibold">Workflow Performance</h3>
            <p className="text-sm text-content-muted">Completion rates and average step durations.</p>
          </div>
          <div className="flex gap-4 text-center text-sm">
            <div>
              <div className="font-mono text-xl font-semibold text-brand-primary">
                {Math.round(workflowAnalytics?.completion_rate ?? 0)}%
              </div>
              <div className="text-xs text-content-muted">Completion</div>
            </div>
            <div>
              <div className="font-mono text-xl font-semibold">
                {formatDuration(workflowAnalytics?.avg_duration_ms ?? 0)}
              </div>
              <div className="text-xs text-content-muted">Avg Duration</div>
            </div>
            <div>
              <div className="font-mono text-xl font-semibold">{compactNumber(workflowAnalytics?.total_workflows ?? 0)}</div>
              <div className="text-xs text-content-muted">Total Runs</div>
            </div>
          </div>
        </div>
        <div className="h-56">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={workflowAnalytics?.step_stats ?? []} layout="vertical">
              <CartesianGrid stroke="rgba(148, 163, 184, 0.1)" horizontal={false} />
              <XAxis type="number" stroke="#94a3b8" fontSize={11} tickFormatter={(v) => `${Math.round(v / 1000)}s`} />
              <YAxis type="category" dataKey="step" stroke="#94a3b8" fontSize={11} width={100} />
              <Tooltip
                contentStyle={{ background: "#0f172a", border: "1px solid rgba(148,163,184,0.22)", borderRadius: 8, fontSize: 12 }}
                // eslint-disable-next-line @typescript-eslint/no-explicit-any
                formatter={(value: any) => [`${Math.round(Number(value) / 1000)}s`, "Avg Duration"]}
              />
              <Bar dataKey="avg_ms" fill="var(--color-brand-primary)" radius={[0, 6, 6, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </section>

      {/* Agent Performance Table */}
      <section className="overflow-hidden rounded-lg border border-stroke bg-panel">
        <div className="border-b border-stroke p-5">
          <h3 className="font-mono font-semibold">Agent Performance</h3>
          <p className="text-sm text-content-muted">Per-agent metrics, success rates, and token consumption.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
              <tr>
                <th className="px-4 py-3">Agent</th>
                <th className="px-4 py-3">Role</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Tasks</th>
                <th className="px-4 py-3">Success</th>
                <th className="px-4 py-3">Retries</th>
                <th className="px-4 py-3">Tokens</th>
                <th className="px-4 py-3">Cost</th>
              </tr>
            </thead>
            <tbody>
              {agentStats?.map((agent) => (
                <tr key={agent.agent_id} className="border-b border-stroke/60 transition hover:bg-slate-900/50">
                  <td className="px-4 py-3">
                    <div className="font-medium">{agent.agent_name}</div>
                    <div className="text-xs text-content-muted">{agent.model_route}</div>
                  </td>
                  <td className="px-4 py-3 capitalize">{agent.role}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium ${agent.status === "idle" ? "bg-slate-700/50 text-slate-300" :
                      agent.status === "busy" || agent.status === "running" ? "bg-emerald-400/10 text-emerald-300" :
                        "bg-slate-700/50 text-slate-400"
                      }`}>
                      <span className={`size-1.5 rounded-full ${agent.status === "idle" ? "bg-slate-400" :
                        agent.status === "busy" || agent.status === "running" ? "bg-emerald-400 animate-pulse" :
                          "bg-slate-500"
                        }`} />
                      {agent.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono">{agent.task_count}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className="h-1.5 w-16 rounded-full bg-slate-700">
                        <div
                          className="h-1.5 rounded-full bg-brand-primary transition-all"
                          style={{ width: `${Math.min(agent.success_rate, 100)}%` }}
                        />
                      </div>
                      <span className="text-xs font-mono">{Math.round(agent.success_rate)}%</span>
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono">{agent.retry_count}</td>
                  <td className="px-4 py-3 font-mono">{compactNumber(agent.total_tokens)}</td>
                  <td className="px-4 py-3 font-mono">{formatCost(agent.total_cost_usd)}</td>
                </tr>
              ))}
              {agentStats?.length === 0 && (
                <tr>
                  <td className="px-4 py-8 text-center text-content-muted" colSpan={8}>
                    No agent data available yet.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </DashboardLayout>
  );
}

function StatCard({ icon: Icon, label, value, accent }: { icon: LucideIcon; label: string; value: string; accent?: boolean }) {
  return (
    <article className="group rounded-lg border border-stroke bg-panel p-4 transition hover:border-brand-primary/40">
      <div className="mb-2 grid size-8 place-items-center rounded-md bg-brand-primary/10 text-brand-primary">
        <Icon size={16} />
      </div>
      <div className={`font-mono text-xl font-semibold transition ${accent ? "text-brand-primary" : "group-hover:text-brand-primary"}`}>
        {value}
      </div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}
