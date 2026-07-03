"use client";

import Link from "next/link";
import { useMemo, useState } from "react";
import {
  AreaChart,
  Area,
  BarChart,
  Bar,
  CartesianGrid,
  Cell,
  Legend,
  PieChart,
  Pie,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import {
  Activity,
  AlertTriangle,
  Bot,
  CheckCircle2,
  Clock,
  Coins,
  FolderKanban,
  GitPullRequest,
  Timer,
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
function formatLatency(ms: number) {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
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
  const [selectedProjectID, setSelectedProjectID] = useState("");

  const { data: projects = [] } = useAuthedSWR(
    orgID ? ["analytics-projects", orgID] : null,
    (token) => api.listProjects(orgID, token),
  );

  const { data: overview } = useAuthedSWR(
    orgID ? ["analytics-overview", orgID] : null,
    (token) => api.analyticsOverview(token, orgID),
  );
  const { data: agentStats = [] } = useAuthedSWR(
    orgID ? ["analytics-agents", orgID, selectedProjectID] : null,
    (token) => api.analyticsAgents(token, orgID, selectedProjectID || undefined),
  );
  const { data: taskAnalytics } = useAuthedSWR(
    orgID ? ["analytics-tasks", orgID, selectedProjectID] : null,
    (token) => api.analyticsTasks(token, orgID, selectedProjectID || undefined),
  );
  const { data: gatewayUsage = [] } = useAuthedSWR(
    orgID ? ["analytics-gateway-usage", orgID, selectedProjectID] : null,
    (token) => api.analyticsGatewayUsage(token, orgID, selectedProjectID || undefined),
  );
  const { data: workflowAnalytics } = useAuthedSWR(
    orgID ? ["analytics-workflows", orgID, selectedProjectID] : null,
    (token) => api.analyticsWorkflows(token, orgID, selectedProjectID || undefined),
  );
  const { data: recentFailures = [] } = useAuthedSWR(
    orgID ? ["analytics-failures", orgID, selectedProjectID] : null,
    (token) => api.analyticsFailures(token, orgID, selectedProjectID || undefined, 5),
  );
  const { data: tokenUsageList = [] } = useAuthedSWR(
    orgID ? ["analytics-token-usage", orgID, selectedProjectID] : null,
    (token) => api.tokenUsage(token, orgID, 30, selectedProjectID || undefined),
  );

  const avgLatencyMs = useMemo(() => {
    const totals = tokenUsageList.reduce(
      (acc, item) => {
        acc.requests += item.requests || 0;
        acc.latency += (item.avg_latency_ms || 0) * (item.requests || 0);
        return acc;
      },
      { requests: 0, latency: 0 },
    );
    return totals.requests > 0 ? totals.latency / totals.requests : 0;
  }, [tokenUsageList]);

  const keyLabelUsage = useMemo(() => {
    const groups: Record<string, {
      key_label: string;
      success: number;
      failed: number;
      total_tokens: number;
      cost_usd: number;
    }> = {};

    tokenUsageList.forEach((item) => {
      const label = item.key_label || "No Label (Gateway)";
      if (!groups[label]) {
        groups[label] = {
          key_label: label,
          success: 0,
          failed: 0,
          total_tokens: 0,
          cost_usd: 0,
        };
      }
      groups[label].success += item.success_requests || 0;
      groups[label].failed += item.failed_requests || 0;
      groups[label].total_tokens += item.total_tokens || 0;
      groups[label].cost_usd += item.cost_usd || 0;
    });

    return Object.values(groups).sort((a, b) => b.cost_usd - a.cost_usd);
  }, [tokenUsageList]);

  const providerUsage = useMemo(() => {
    const groups: Record<string, {
      provider: string;
      model: string;
      level_group: string;
      requests: number;
      total_tokens: number;
      cost_usd: number;
      avg_latency_ms: number;
    }> = {};

    tokenUsageList.forEach((item) => {
      const key = `${item.provider}:${item.model}:${item.level_group}`;
      if (!groups[key]) {
        groups[key] = {
          provider: item.provider,
          model: item.model,
          level_group: item.level_group,
          requests: 0,
          total_tokens: 0,
          cost_usd: 0,
          avg_latency_ms: 0,
        };
      }
      const current = groups[key];
      const nextRequests = current.requests + (item.requests || 0);
      current.avg_latency_ms = nextRequests > 0
        ? ((current.avg_latency_ms * current.requests) + ((item.avg_latency_ms || 0) * (item.requests || 0))) / nextRequests
        : 0;
      current.requests = nextRequests;
      current.total_tokens += item.total_tokens || 0;
      current.cost_usd += item.cost_usd || 0;
    });

    return Object.values(groups).sort((a, b) => b.cost_usd - a.cost_usd);
  }, [tokenUsageList]);

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="font-mono text-2xl font-semibold">Analytics</h1>
          <p className="mt-1 text-sm text-content-muted">
            Platform performance, agent metrics, and workflow health.
          </p>
        </div>
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <select
            value={selectedProjectID}
            onChange={(event) => setSelectedProjectID(event.target.value)}
            className="rounded-md border border-stroke bg-panel px-3 py-2 text-sm text-slate-200 focus:border-brand-primary focus:outline-none"
          >
            <option value="">All projects</option>
            {projects.map((project) => (
              <option key={project.id} value={project.id}>{project.name}</option>
            ))}
          </select>
          <div className="rounded-full border border-stroke bg-panel px-3 py-1 text-xs text-content-muted">
            Phase 5 dashboard
          </div>
        </div>
      </div>

      {/* Overview stat cards */}
      <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-8">
        <StatCard icon={FolderKanban} label="Projects" value={compactNumber(overview?.total_projects ?? 0)} />
        <StatCard icon={Activity} label="Active Tasks" value={compactNumber(overview?.active_tasks ?? 0)} accent />
        <StatCard icon={CheckCircle2} label="Success Rate" value={`${Math.round(overview?.success_rate ?? 0)}%`} />
        <StatCard icon={Bot} label="Running Agents" value={`${overview?.running_agents ?? 0} / ${overview?.total_agents ?? 0}`} />
        <StatCard icon={GitPullRequest} label="Open PRs" value={compactNumber(overview?.open_prs ?? 0)} />
        <StatCard icon={Timer} label="Avg Cycle" value={formatDuration(overview?.avg_completion_ms ?? 0)} />
        <StatCard icon={Clock} label="Avg Latency" value={formatLatency(avgLatencyMs)} />
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
                formatter={(value: unknown) => [`${Math.round(Number(value) / 1000)}s`, "Avg Duration"]}
              />
              <Bar dataKey="avg_ms" fill="var(--color-brand-primary)" radius={[0, 6, 6, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </section>

      {/* Virtual Key Analytics */}
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

      {/* Gateway Usage Trend */}
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
                formatter={(value, name) => {
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

      {/* Provider Analytics */}
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

      {/* Recent Failures */}
      <section className="mb-6 overflow-hidden rounded-lg border border-stroke bg-panel">
        <div className="border-b border-stroke p-5">
          <div className="flex items-center gap-2">
            <AlertTriangle size={17} className="text-red-400" />
            <h3 className="font-mono font-semibold">Recent Failures</h3>
          </div>
          <p className="mt-1 text-sm text-content-muted">Latest failed tasks and the last workflow error recorded for each one.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
              <tr>
                <th className="px-4 py-3">Task</th>
                <th className="px-4 py-3">Project</th>
                <th className="px-4 py-3">Step</th>
                <th className="px-4 py-3">Reason</th>
                <th className="px-4 py-3">Failed</th>
              </tr>
            </thead>
            <tbody>
               {(recentFailures || []).map((failure) => (
                <tr key={failure.task_id} className="border-b border-stroke/60 transition hover:bg-slate-900/50">
                  <td className="px-4 py-3">
                    <Link href={`/projects/${failure.project_id}/tasks/${failure.task_id}`} className="font-medium text-white hover:text-brand-primary">
                      {failure.title}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-content-muted">{failure.project_name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-content-muted">{failure.workflow_step || "unknown"}</td>
                  <td className="max-w-xl px-4 py-3 text-content-muted">
                    <span className="line-clamp-2">{failure.failure_reason || "No workflow error recorded."}</span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-content-muted">
                    {new Date(failure.failed_at).toLocaleString()}
                  </td>
                </tr>
              ))}
              {(!recentFailures || recentFailures.length === 0) && (
                <tr>
                  <td className="px-4 py-8 text-center text-content-muted" colSpan={5}>
                    No failed tasks recorded.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
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
                    <div className="mt-1">
                      <span className={`inline-flex items-center gap-0.5 rounded px-1.5 py-0.2 text-[9px] font-bold uppercase ${
                        agent.model_level_group === "fast"
                          ? "bg-amber-500/10 text-amber-500 border border-amber-500/20"
                          : agent.model_level_group === "powerful"
                          ? "bg-purple-500/10 text-purple-500 border border-purple-500/20"
                          : "bg-blue-500/10 text-blue-500 border border-blue-500/20"
                      }`}>
                        {agent.model_level_group === "fast" && "⚡ "}
                        {agent.model_level_group === "balanced" && "⚖️ "}
                        {agent.model_level_group === "powerful" && "🚀 "}
                        {agent.model_level_group}
                      </span>
                    </div>
                  </td>
                  <td className="px-4 py-3 capitalize">{agent.role}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium ${
                      agent.status === "idle" ? "bg-slate-700/20 text-slate-400 border-slate-700/30" :
                      agent.status === "assigned" ? "bg-blue-500/10 text-blue-300 border-blue-500/20" :
                      agent.status === "running" || agent.status === "busy" ? "bg-emerald-400/10 text-emerald-300 border-emerald-400/20" :
                      agent.status === "offline" ? "bg-red-500/10 text-red-400 border-red-500/20" :
                      "bg-slate-700/20 text-slate-400 border-slate-700/30"
                    }`}>
                      <span className={`size-1.5 rounded-full ${
                        agent.status === "idle" ? "bg-slate-500" :
                        agent.status === "assigned" ? "bg-blue-400 animate-pulse" :
                        agent.status === "running" || agent.status === "busy" ? "bg-emerald-400 animate-pulse" :
                        agent.status === "offline" ? "bg-red-500" :
                        "bg-slate-500"
                      }`} />
                      {agent.status || "idle"}
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
