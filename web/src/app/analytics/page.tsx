"use client";

import { useState } from "react";
import {
  Activity,
  Bot,
  CheckCircle2,
  FolderKanban,
  GitPullRequest,
  Timer,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { compactNumber, formatDuration } from "./utils";

import { StatCard } from "./components/StatCard";
import { TaskCharts } from "./components/TaskCharts";
import { WorkflowAnalytics } from "./components/WorkflowAnalytics";
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
  const { data: workflowAnalytics } = useAuthedSWR(
    orgID ? ["analytics-workflows", orgID, selectedProjectID] : null,
    (token) => api.analyticsWorkflows(token, orgID, selectedProjectID || undefined),
  );
  const { data: recentFailures = [] } = useAuthedSWR(
    orgID ? ["analytics-failures", orgID, selectedProjectID] : null,
    (token) => api.analyticsFailures(token, orgID, selectedProjectID || undefined, 5),
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
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <select
            value={selectedProjectID}
            onChange={(event) => setSelectedProjectID(event.target.value)}
            className="rounded-md border border-stroke bg-panel px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none"
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
      <div className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6">
        <StatCard icon={FolderKanban} label="Projects" value={compactNumber(overview?.total_projects ?? 0)} />
        <StatCard icon={Activity} label="Active Tasks" value={compactNumber(overview?.active_tasks ?? 0)} accent />
        <StatCard icon={CheckCircle2} label="Success Rate" value={`${Math.round(overview?.success_rate ?? 0)}%`} />
        <StatCard icon={Bot} label="Running Agents" value={`${overview?.running_agents ?? 0} / ${overview?.total_agents ?? 0}`} />
        <StatCard icon={GitPullRequest} label="Open PRs" value={compactNumber(overview?.open_prs ?? 0)} />
        <StatCard icon={Timer} label="Avg Cycle" value={formatDuration(overview?.avg_completion_ms ?? 0)} />
      </div>

      <TaskCharts taskAnalytics={taskAnalytics} />
      <WorkflowAnalytics workflowAnalytics={workflowAnalytics} />
      <RecentFailures recentFailures={recentFailures} />
      <AgentPerformanceTable agentStats={agentStats} />
    </DashboardLayout>
  );
}
