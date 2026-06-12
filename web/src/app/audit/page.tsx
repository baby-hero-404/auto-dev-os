"use client";

import { useState, Fragment } from "react";
import useSWR from "swr";
import {
  Bot,
  ChevronDown,
  ChevronUp,
  Cpu,
  Download,
  FileCode,
  Globe,
  Search,
  ShieldAlert,
  ShieldCheck,
  User,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { AuditLog } from "@/lib/types";

const SEVERITY_COLORS: Record<string, string> = {
  "task.created": "bg-slate-500/10 text-slate-400 border-slate-500/20",
  "task.status_changed": "bg-blue-500/10 text-blue-400 border-blue-500/20",
  "task.executed": "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  "pr.created": "bg-purple-500/10 text-purple-400 border-purple-500/20",
  "pr.approved": "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  "pr.rejected": "bg-amber-500/10 text-amber-400 border-amber-500/20",
  "pr.merged": "bg-teal-500/10 text-teal-400 border-teal-500/20",
  "agent.assigned": "bg-indigo-500/10 text-indigo-400 border-indigo-500/20",
  "secret.accessed": "bg-red-500/10 text-red-400 border-red-500/20 animate-pulse",
  "rule.modified": "bg-orange-500/10 text-orange-400 border-orange-500/20",
  "workflow.started": "bg-sky-500/10 text-sky-400 border-sky-500/20",
  "workflow.completed": "bg-emerald-500/10 text-emerald-400 border-emerald-500/20",
  "workflow.failed": "bg-red-500/10 text-red-400 border-red-500/20",
};

export default function AuditLogPage() {
  const session = useSession();
  const token = session?.token ?? "";

  const [selectedAction, setSelectedAction] = useState("");
  const [selectedEntity, setSelectedEntity] = useState("");
  const [days, setDays] = useState(30);
  const [searchQuery, setSearchQuery] = useState("");
  const [expandedLog, setExpandedLog] = useState<string | null>(null);

  // Fetch summary count
  const { data: summary = {} } = useAuthedSWR(
    ["audit-summary"],
    (t) => api.auditSummary(t),
  );

  // Fetch audit logs
  const { data: logs = [], error } = useSWR(
    session
      ? [
          "audit-logs",
          selectedAction,
          selectedEntity,
          days,
        ]
      : null,
    () =>
      api.auditLogs(token, {
        action: selectedAction || undefined,
        entity_type: selectedEntity || undefined,
        days,
        limit: 100,
      }),
  );

  // Filtering on-the-fly for search query
  const filteredLogs = logs.filter((log: AuditLog) => {
    if (!searchQuery) return true;
    const query = searchQuery.toLowerCase();
    return (
      log.id.toLowerCase().includes(query) ||
      (log.task_id?.toLowerCase() || "").includes(query) ||
      (log.agent_id?.toLowerCase() || "").includes(query) ||
      log.action.toLowerCase().includes(query) ||
      log.entity_id.toLowerCase().includes(query) ||
      JSON.stringify(log.details).toLowerCase().includes(query)
    );
  });

  const totalEvents = Object.values(summary).reduce((acc, count) => acc + count, 0);
  const highRiskEvents = (summary["secret.accessed"] ?? 0) + (summary["rule.modified"] ?? 0) + (summary["pr.rejected"] ?? 0);

  function exportAuditCSV() {
    const headers = ["Timestamp", "Action", "Actor", "Entity Type", "Entity ID", "IP Address", "Details"];
    const rows = filteredLogs.map((log: AuditLog) => [
      new Date(log.created_at).toISOString(),
      log.action,
      log.agent_id ? `Agent: ${log.agent_id}` : log.user_id ? `User: ${log.user_id}` : "System",
      log.entity_type,
      log.entity_id,
      log.ip_address,
      JSON.stringify(log.details),
    ]);

    const csvContent = [headers.join(","), ...rows.map((e) => e.map((val) => `"${val.replace(/"/g, '""')}"`).join(","))].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `autocodeos_audit_log_${new Date().toISOString().split("T")[0]}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
        <div>
          <h1 className="font-mono text-2xl font-semibold">Audit Log</h1>
          <p className="mt-1 text-sm text-content-muted">
            Immutable trace log of all critical workspace activities and security events.
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={exportAuditCSV}
            disabled={filteredLogs.length === 0}
            className="flex items-center gap-2 rounded-md border border-stroke bg-panel px-3 py-2 text-sm text-slate-200 transition hover:bg-slate-900 disabled:opacity-50"
          >
            <Download size={15} />
            Export CSV
          </button>
          <div className="rounded-full border border-stroke bg-panel px-3 py-1 text-xs text-content-muted flex items-center gap-1.5">
            <ShieldCheck size={14} className="text-emerald-400" />
            Compliance Active
          </div>
        </div>
      </div>

      {/* Summary dashboard */}
      <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border border-stroke bg-panel p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-content-muted">Total Events</div>
          <div className="mt-2 font-mono text-3xl font-bold">{totalEvents}</div>
          <div className="mt-1 text-xs text-content-muted">Across all components</div>
        </div>
        <div className="rounded-lg border border-stroke bg-panel p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-content-muted">Security & Policy</div>
          <div className="mt-2 font-mono text-3xl font-bold text-amber-400">{highRiskEvents}</div>
          <div className="mt-1 text-xs text-content-muted">Secrets accessed & rule edits</div>
        </div>
        <div className="rounded-lg border border-stroke bg-panel p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-content-muted">PR Workflows</div>
          <div className="mt-2 font-mono text-3xl font-bold text-sky-400">
            {(summary["pr.created"] ?? 0) + (summary["pr.approved"] ?? 0)}
          </div>
          <div className="mt-1 text-xs text-content-muted">Reviews & merge actions</div>
        </div>
        <div className="rounded-lg border border-stroke bg-panel p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-content-muted">Agent Failures</div>
          <div className="mt-2 font-mono text-3xl font-bold text-red-400">{summary["workflow.failed"] ?? 0}</div>
          <div className="mt-1 text-xs text-content-muted">Failed runs needing review</div>
        </div>
      </div>

      {/* Filters & search */}
      <div className="mb-6 grid gap-4 rounded-lg border border-stroke bg-panel p-4 md:grid-cols-[1fr_180px_180px_140px]">
        <div className="relative">
          <Search className="absolute left-3 top-2.5 text-content-muted" size={16} />
          <input
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search by action, ID, keyword..."
            className="w-full rounded-md border border-stroke bg-slate-950/50 py-2 pl-10 pr-4 text-sm text-white placeholder-slate-500 focus:border-brand-primary focus:outline-none"
          />
        </div>
        <select
          value={selectedAction}
          onChange={(e) => setSelectedAction(e.target.value)}
          className="rounded-md border border-stroke bg-slate-950/50 px-3 py-2 text-sm text-slate-300 focus:border-brand-primary focus:outline-none"
        >
          <option value="">All Actions</option>
          <option value="task.created">Task Created</option>
          <option value="task.executed">Task Executed</option>
          <option value="pr.created">PR Created</option>
          <option value="pr.approved">PR Approved</option>
          <option value="pr.rejected">PR Rejected</option>
          <option value="secret.accessed">Secret Accessed</option>
          <option value="rule.modified">Rule Modified</option>
          <option value="workflow.failed">Workflow Failed</option>
        </select>
        <select
          value={selectedEntity}
          onChange={(e) => setSelectedEntity(e.target.value)}
          className="rounded-md border border-stroke bg-slate-950/50 px-3 py-2 text-sm text-slate-300 focus:border-brand-primary focus:outline-none"
        >
          <option value="">All Entity Types</option>
          <option value="task">Tasks</option>
          <option value="agent">Agents</option>
          <option value="rule">Rules</option>
          <option value="secret">Secrets</option>
          <option value="workflow">Workflows</option>
        </select>
        <select
          value={days}
          onChange={(e) => setDays(Number(e.target.value))}
          className="rounded-md border border-stroke bg-slate-950/50 px-3 py-2 text-sm text-slate-300 focus:border-brand-primary focus:outline-none"
        >
          <option value="1">Last 24h</option>
          <option value="7">Last 7 days</option>
          <option value="30">Last 30 days</option>
          <option value="90">Last 90 days</option>
        </select>
      </div>

      {error && (
        <p className="mb-4 rounded-md border border-red-400/40 bg-red-950/40 p-3 text-sm text-red-100">
          Failed to load audit logs: {error.message}
        </p>
      )}

      {/* Audit logs stream */}
      <div className="overflow-hidden rounded-lg border border-stroke bg-panel">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted bg-slate-950/30">
              <tr>
                <th className="px-4 py-3.5 w-[180px]">Timestamp</th>
                <th className="px-4 py-3.5 w-[160px]">Actor</th>
                <th className="px-4 py-3.5 w-[200px]">Action</th>
                <th className="px-4 py-3.5 w-[180px]">Target</th>
                <th className="px-4 py-3.5 w-[130px]">IP Address</th>
                <th className="px-4 py-3.5"></th>
              </tr>
            </thead>
            <tbody>
              {filteredLogs.map((log: AuditLog) => {
                const isExpanded = expandedLog === log.id;
                const isAgent = !!log.agent_id;
                const isSecret = log.action === "secret.accessed";

                return (
                  <Fragment key={log.id}>
                    <tr
                      onClick={() => setExpandedLog(isExpanded ? null : log.id)}
                      className={`border-b border-stroke/60 cursor-pointer transition hover:bg-slate-900/50 ${
                        isExpanded ? "bg-slate-900/20" : ""
                      }`}
                    >
                      <td className="px-4 py-3.5 font-mono text-xs text-content-muted">
                        {new Date(log.created_at).toLocaleString()}
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="flex items-center gap-2">
                          {isAgent ? (
                            <>
                              <Bot size={15} className="text-brand-primary" />
                              <span className="font-mono text-xs max-w-[120px] truncate" title={log.agent_id}>
                                Agent
                              </span>
                            </>
                          ) : log.user_id ? (
                            <>
                              <User size={15} className="text-blue-400" />
                              <span className="font-mono text-xs max-w-[120px] truncate" title={log.user_id}>
                                User
                              </span>
                            </>
                          ) : (
                            <>
                              <Cpu size={15} className="text-slate-400" />
                              <span className="font-mono text-xs">System</span>
                            </>
                          )}
                        </div>
                      </td>
                      <td className="px-4 py-3.5">
                        <span className={`inline-flex rounded border px-2 py-0.5 text-xs font-semibold font-mono ${
                          SEVERITY_COLORS[log.action] ?? "bg-slate-800 text-slate-300 border-slate-700"
                        }`}>
                          {log.action}
                        </span>
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="flex items-center gap-1.5 font-mono text-xs">
                          <FileCode size={13} className="text-content-muted" />
                          <span className="text-content-muted">{log.entity_type}:</span>
                          <span className="text-slate-300 truncate max-w-[100px]" title={log.entity_id}>
                            {log.entity_id || "global"}
                          </span>
                        </div>
                      </td>
                      <td className="px-4 py-3.5 font-mono text-xs text-content-muted">
                        <span className="flex items-center gap-1">
                          <Globe size={12} className="text-slate-500" />
                          {log.ip_address || "local"}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 text-right">
                        {isExpanded ? (
                          <ChevronUp size={16} className="inline text-content-muted" />
                        ) : (
                          <ChevronDown size={16} className="inline text-content-muted" />
                        )}
                      </td>
                    </tr>
                    {isExpanded && (
                      <tr className="bg-slate-950/40 border-b border-stroke/40">
                        <td colSpan={6} className="p-4">
                          <div className="rounded-md border border-stroke/50 bg-slate-950 p-4">
                            <div className="mb-2.5 flex items-center justify-between text-xs text-content-muted">
                              <span className="font-mono">Event metadata ID: {log.id}</span>
                              {isSecret && (
                                <span className="flex items-center gap-1 text-red-400 font-semibold uppercase tracking-wider">
                                  <ShieldAlert size={14} /> Security Alert
                                </span>
                              )}
                            </div>
                            <pre className="overflow-x-auto font-mono text-xs text-emerald-400/90 leading-relaxed">
                              {JSON.stringify(log.details, null, 2)}
                            </pre>
                            {log.task_id && (
                              <div className="mt-3.5 flex justify-end">
                                <a
                                  href={`/tasks/${log.task_id}`}
                                  className="text-xs font-semibold text-brand-primary hover:underline flex items-center gap-1.5"
                                >
                                  Go to Task Workflow &rarr;
                                </a>
                              </div>
                            )}
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
              {filteredLogs.length === 0 && (
                <tr>
                  <td className="px-4 py-12 text-center text-content-muted font-mono text-sm" colSpan={6}>
                    No audit records match the current filters.
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>
    </DashboardLayout>
  );
}
