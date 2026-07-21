"use client";

import { useMemo, useState } from "react";
import {
  Activity,
  Bot,
  CheckCircle2,
  Clock,
  Coins,
  FolderKanban,
  GitPullRequest,
  Timer,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { compactNumber, formatCost, formatDuration, formatLatency } from "./utils";

import { StatCard } from "./components/StatCard";
import { TaskCharts } from "./components/TaskCharts";
import { WorkflowAnalytics } from "./components/WorkflowAnalytics";
import { VirtualKeyAnalytics } from "./components/VirtualKeyAnalytics";
import { GatewayUsageTrend } from "./components/GatewayUsageTrend";
import { ProviderAnalytics } from "./components/ProviderAnalytics";
import { RecentFailures } from "./components/RecentFailures";
import { AgentPerformanceTable } from "./components/AgentPerformanceTable";

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
      key_label?: string;
      requests: number;
      total_tokens: number;
      cost_usd: number;
      avg_latency_ms: number;
    }> = {};

    tokenUsageList.forEach((item) => {
      const label = item.key_label || "No Label";
      const key = `${item.provider}:${item.model}:${label}:${item.level_group}`;
      if (!groups[key]) {
        groups[key] = {
          provider: item.provider,
          model: item.model,
          level_group: item.level_group,
          key_label: label,
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

      <TaskCharts taskAnalytics={taskAnalytics} />
      <WorkflowAnalytics workflowAnalytics={workflowAnalytics} />
      <VirtualKeyAnalytics keyLabelUsage={keyLabelUsage} />
      <GatewayUsageTrend gatewayUsage={gatewayUsage} />
      <ProviderAnalytics providerUsage={providerUsage} />
      <RecentFailures recentFailures={recentFailures} />
      <AgentPerformanceTable agentStats={agentStats} />
    </DashboardLayout>
  );
}
