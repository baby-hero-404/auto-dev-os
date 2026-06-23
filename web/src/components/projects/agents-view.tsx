"use client";

import { FormEvent, useState } from "react";

import type { Agent } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { api, ApiError } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";

type AgentsViewProps = {
  orgID: string;
  projectAgents: Agent[];
  isLoading: boolean;
  onAssignAgent: (staff: Agent) => Promise<void>;
};

export function AgentsView({
  orgID,
  projectAgents,
  isLoading,
  onAssignAgent,
}: AgentsViewProps) {
  const [selectedStaff, setSelectedStaff] = useState("");
  const [assignError, setAssignError] = useState("");
  const [isAssigning, setIsAssigning] = useState(false);

  // Fetch organization agents
  const orgAgentsSWR = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t)
  );
  const orgAgents = orgAgentsSWR.data || [];
  const isOrgAgentsLoading = orgAgentsSWR.isLoading;

  const assignableStaff = orgAgents.filter(
    (staff) =>
      staff.assignment_strategy !== "auto_join" &&
      !projectAgents.some((pa) => pa.id === staff.id)
  );
  const inheritedAgents = projectAgents.filter((a) => a.assignment_strategy === "auto_join");
  const manualAgents = projectAgents.filter((a) => a.assignment_strategy !== "auto_join");

  async function handleAssign(e: FormEvent) {
    e.preventDefault();
    if (!selectedStaff) return;
    const staff = orgAgents.find((s) => s.id === selectedStaff);
    if (!staff) return;
    setAssignError("");
    setIsAssigning(true);
    try {
      await onAssignAgent(staff);
      setSelectedStaff("");
    } catch (err) {
      setAssignError(err instanceof ApiError ? err.message : "Failed to assign agent");
    } finally {
      setIsAssigning(false);
    }
  }

  return (
    <div className="space-y-6">
      {/* Metric Cards */}
      <div className="grid gap-3 sm:grid-cols-3">
        <MetricCard label="Total Members" value={projectAgents.length} />
        <MetricCard label="Inherited (Global)" value={inheritedAgents.length} />
        <MetricCard label="Project-specific" value={manualAgents.length} />
      </div>

      <div className="grid gap-6">
        {/* Left column: Inherited & Project-specific lists */}
        <div className="space-y-6">
          {/* Inherited (auto_join) agents */}
          <section className="rounded-lg border border-stroke bg-card p-5">
            <div className="mb-4 flex items-center justify-between gap-3 border-b border-stroke pb-3">
              <div>
                <h3 className="font-sans font-semibold text-foreground">Inherited from Global</h3>
                <p className="text-xs text-content-muted mt-1">
                  Global agents automatically join every project.
                </p>
              </div>
              <Badge value="auto_join" />
            </div>

            {isLoading || isOrgAgentsLoading ? (
              <MembersSkeleton />
            ) : inheritedAgents.length === 0 ? (
              <p className="text-xs italic text-content-muted py-2">
                No auto-join agents in the organization pool.
              </p>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2">
                {inheritedAgents.map((agent) => (
                  <AgentCard
                    key={agent.id}
                    agent={agent}
                    badge="Global"
                  />
                ))}
              </div>
            )}
          </section>

          {/* Project-specific (manual) agents */}
          <section className="rounded-lg border border-stroke bg-card p-5">
            <div className="mb-4 flex items-center justify-between gap-3 border-b border-stroke pb-3">
              <div>
                <h3 className="font-sans font-semibold text-foreground">Project-specific</h3>
                <p className="text-xs text-content-muted mt-1">
                  Manual agent assignments assigned to this workspace.
                </p>
              </div>
              <Badge value="manual" />
            </div>

            {isLoading || isOrgAgentsLoading ? (
              <MembersSkeleton />
            ) : manualAgents.length === 0 ? (
              <p className="text-xs italic text-content-muted py-2">
                No manual agents assigned to this project yet.
              </p>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2">
                {manualAgents.map((agent) => (
                  <AgentCard
                    key={agent.id}
                    agent={agent}
                    badge="Manual"
                  />
                ))}
              </div>
            )}

            {/* Assign agent form */}
            <form className="mt-5 border-t border-stroke pt-4" onSubmit={handleAssign}>
              <label className="mb-1.5 block font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
                Assign Agent from Organization Pool
              </label>
              <div className="flex flex-col gap-2 sm:flex-row">
                <select
                  value={selectedStaff}
                  onChange={(e) => setSelectedStaff(e.target.value)}
                  className="min-w-0 flex-1 rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none cursor-pointer"
                  disabled={isAssigning || assignableStaff.length === 0}
                >
                  <option value="">
                    {assignableStaff.length === 0 ? "No manual agents available" : "Choose a manual agent..."}
                  </option>
                  {assignableStaff.map((staff) => (
                    <option key={staff.id} value={staff.id}>
                      {staff.name} ({staff.role})
                    </option>
                  ))}
                </select>
                <button
                  disabled={!selectedStaff || isAssigning}
                  className="rounded bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer whitespace-nowrap"
                  type="submit"
                >
                  {isAssigning ? "Assigning..." : "Assign Agent"}
                </button>
              </div>
              {assignError && <p className="mt-2 text-xs text-red-400">{assignError}</p>}
            </form>
          </section>
        </div>
      </div>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: number }) {
  return (
    <article className="rounded-lg border border-stroke bg-card p-4">
      <div className="font-sans text-xl font-bold text-foreground">{value}</div>
      <div className="text-xs text-content-muted mt-0.5">{label}</div>
    </article>
  );
}

function AgentCard({
  agent,
  badge,
}: {
  agent: Agent;
  badge: string;
}) {
  const initials = agent.name
    .split(/\s+/)
    .map((n) => n[0])
    .join("")
    .slice(0, 2)
    .toUpperCase();

  return (
    <article className="rounded-lg border border-stroke bg-surface/30 p-4 transition hover:border-brand-primary/30">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="flex h-7 w-7 items-center justify-center rounded-full bg-brand-primary/10 border border-brand-primary/20 text-[11px] font-bold text-brand-primary">
            {initials}
          </div>
          <div className="min-w-0">
            <h4 className="truncate font-sans font-semibold text-foreground">{agent.name}</h4>
            <div className="mt-1 flex items-center gap-1">
              <span className={`inline-flex items-center gap-1 rounded-full px-1.5 py-0.5 text-[9px] font-bold uppercase ${
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
          </div>
        </div>
        <span className="rounded border border-stroke bg-card px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted shrink-0">
          {badge}
        </span>
      </div>
      <div className="flex flex-wrap gap-1.5 pt-1">
        <Badge value={agent.role} />
        <Badge value={agent.autonomy_level || "supervised"} />
      </div>
    </article>
  );
}

function MembersSkeleton() {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {[0, 1].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-surface/30 p-4">
          <div className="skeleton-shimmer h-5 w-40 rounded" />
          <div className="mt-2 skeleton-shimmer h-3 w-28 rounded" />
          <div className="mt-4 flex gap-2">
            <div className="skeleton-shimmer h-5 w-16 rounded" />
            <div className="skeleton-shimmer h-5 w-20 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
