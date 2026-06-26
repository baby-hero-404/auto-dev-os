"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import { ArrowRight, Bot, ChevronDown, Loader2, Plus, Sparkles, Trash2 } from "lucide-react";
import { Toaster, toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Agent } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { HireAgentWizard, type HireAgentPayload } from "@/components/dashboard/hire-agent-wizard";

const DEFAULT_FLEET = [
  {
    name: "AI Planner",
    role: "planner",
    goal: "Break requirements into execution plans, milestones, and task dependencies.",
    model_level_group: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Backend Developer",
    role: "backend",
    goal: "Implement server-side features, persistence, APIs, and integration logic.",
    model_level_group: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Frontend Developer",
    role: "frontend",
    goal: "Build user-facing flows, application state, and responsive interface behavior.",
    model_level_group: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Reviewer",
    role: "reviewer",
    goal: "Review changes for correctness, regressions, security issues, and missing tests.",
    model_level_group: "fast",
    autonomy_level: "approval_required",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI QA Tester",
    role: "qa",
    goal: "Design and run verification checks for functional and regression coverage.",
    model_level_group: "fast",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Security Auditor",
    role: "security-auditor",
    goal: "Scan for vulnerabilities and verify secret safety.",
    model_level_group: "fast",
    autonomy_level: "approval_required",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI DB Architect",
    role: "db-architect",
    goal: "Design schemas, create migrations, and optimize queries.",
    model_level_group: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
];

export function MembersPanel() {
  const session = useSession();
  const [isHireModalOpen, setIsHireModalOpen] = useState(false);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSeeding, setIsSeeding] = useState(false);

  const [assigningMap, setAssigningMap] = useState<Record<string, string>>({});
  const [confirmingAgentID, setConfirmingAgentID] = useState("");
  const [agentActionErrors, setAgentActionErrors] = useState<Record<string, string>>({});

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: projects = [] } = useAuthedSWR(
    orgID ? ["projects", orgID] : null,
    (t) => api.listProjects(orgID, t),
  );

  const { data: orgAgents = [], mutate: mutateOrgAgents, isLoading: isAgentsLoading } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  const { data: roleTemplates = [] } = useAuthedSWR(
    token ? ["role-templates"] : null,
    (t) => api.listRoleTemplates(t),
  );

  const assignmentKey = useMemo(() => {
    if (!session || projects.length === 0) return null;
    const projectIDs = [...projects].sort((a, b) => a.id.localeCompare(b.id)).map((project) => project.id).join(",");
    return ["assignments", projectIDs] as const;
  }, [projects, session]);

  const { data: assignments = {}, mutate: mutateAssignments } = useSWR(
    assignmentKey,
    async ([, idsStr]) => {
      const map: Record<string, string[]> = {};
      await Promise.all(
        idsStr.split(",").map(async (projectID) => {
          try {
            const project = projects.find((item) => item.id === projectID);
            const agents = await api.listAgents(projectID, token);
            agents.forEach((agent) => {
              if (!map[agent.id]) map[agent.id] = [];
              if (project && !map[agent.id].includes(project.name)) {
                map[agent.id].push(project.name);
              }
            });
          } catch (err) {
            console.error("Failed to fetch project agents", projectID, err);
          }
        }),
      );
      return map;
    },
  );

  const manualAgents = orgAgents.filter((agent) => agent.assignment_strategy !== "auto_join").length;
  const autoJoinAgents = orgAgents.length - manualAgents;

  function resetHireModal() {
    setIsHireModalOpen(false);
    setFormError("");
  }

  async function handleHireAgent(payload: HireAgentPayload) {
    if (!token || !orgID) return;

    setFormError("");
    setIsSubmitting(true);
    try {
      await api.hireAgent(orgID, token, payload);
      resetHireModal();
      mutateOrgAgents();
      toast.success(`${payload.name} hired.`);
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to hire agent");
    } finally {
      setIsSubmitting(false);
    }
  }



  async function removeAgent(agentID: string) {
    setAgentActionErrors((prev) => ({ ...prev, [agentID]: "" }));
    try {
      await api.deleteAgent(agentID, token);
      setConfirmingAgentID("");
      mutateOrgAgents();
      mutateAssignments();
    } catch (err) {
      setAgentActionErrors((prev) => ({
        ...prev,
        [agentID]: err instanceof ApiError ? err.message : "Failed to remove agent",
      }));
    }
  }

  async function assignAgentToProject(agent: Agent) {
    const projectID = assigningMap[agent.id];
    if (!projectID || !token) return;
    setAgentActionErrors((prev) => ({ ...prev, [agent.id]: "" }));
    try {
      await api.createAgent(projectID, token, {
        agent_id: agent.id,
        name: agent.name,
        role: agent.role,
        goal: agent.goal,
        autonomy_level: agent.autonomy_level,
        model_level_group: agent.model_level_group,
        assignment_strategy: agent.assignment_strategy,
      });
      setAssigningMap((prev) => {
        const next = { ...prev };
        delete next[agent.id];
        return next;
      });
      mutateAssignments();
    } catch (err) {
      setAgentActionErrors((prev) => ({
        ...prev,
        [agent.id]: err instanceof ApiError ? err.message : "Failed to assign agent to project",
      }));
    }
  }
  async function seedDefaultFleet() {
    if (!token || !orgID || isSeeding || orgAgents.length >= DEFAULT_FLEET.length) return;
    setIsSeeding(true);
    setFormError("");
    let created = 0;
    const existingNames = new Set(orgAgents.map((agent) => agent.name));

    try {
      for (const agent of DEFAULT_FLEET) {
        if (existingNames.has(agent.name)) continue;
        try {
          await api.hireAgent(orgID, token, agent);
          created += 1;
          toast.success(`${agent.name} created.`);
        } catch (err) {
          const message = err instanceof ApiError ? err.message : "Failed to create agent.";
          toast.error(`${agent.name}: ${message}`);
        }
      }
      await mutateOrgAgents();
      await mutateAssignments();
      if (created === 0) {
        toast.info("No new default agents were created.");
      }
    } finally {
      setIsSeeding(false);
    }
  }

  return (
    <div className="space-y-6">
      <Toaster richColors position="top-right" />
      <div className="grid gap-3 md:grid-cols-4">
        <Metric label="Total agents" value={isAgentsLoading ? "--" : orgAgents.length.toString()} />
        <Metric label="Auto-join" value={isAgentsLoading ? "--" : autoJoinAgents.toString()} />
        <Metric label="Manual" value={isAgentsLoading ? "--" : manualAgents.toString()} />
        <Metric
          label="Providers"
          value={
            isAgentsLoading
              ? "--"
              : providerSummary(orgAgents) || "gateway"
          }
        />
      </div>

      <section className="rounded-lg border border-stroke bg-card p-5">
        <div className="mb-4 flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
          <div className="flex items-center gap-2">
            <Bot size={18} className="text-brand-primary" />
            <h3 className="font-mono font-semibold text-foreground">Agent pool actions</h3>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setIsHireModalOpen(true)}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer"
              type="button"
            >
              <Plus size={16} />
              Hire Agent
            </button>
            <button
              onClick={seedDefaultFleet}
              disabled={isSeeding || orgAgents.length >= DEFAULT_FLEET.length}
              title={seedFleetTitle(orgAgents.length)}
              className="inline-flex items-center justify-center gap-2 rounded-md border border-stroke bg-surface px-3 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
              type="button"
            >
              {isSeeding ? <Loader2 size={16} className="animate-spin" /> : <Sparkles size={16} className="text-brand-primary" />}
              Seed Default Fleet
            </button>
          </div>
        </div>
        {formError && <p className="mt-3 rounded border border-red-500/20 bg-red-500/10 p-2 text-xs text-red-600 dark:text-red-400 font-medium">{formError}</p>}
      </section>

      {isAgentsLoading ? (
        <AgentCardsSkeleton />
      ) : orgAgents.length === 0 ? (
        <EmptyState icon={Bot} title="No agents hired yet" description="Create your first capability-based agent or seed the default fleet." />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {orgAgents.map((agent) => {
            const assignedProjectNames = assignments[agent.id] || [];
            const isAutoJoin = agent.assignment_strategy === "auto_join";
            const assignableProjects = projects.filter((project) => !assignedProjectNames.includes(project.name));
            return (
              <article key={agent.id} className="glass-panel glow-on-hover group flex flex-col justify-between rounded-lg p-5">
                <div>
                  <div className="mb-3 flex items-start justify-between gap-3">
                    <div className="flex min-w-0 items-center gap-3">
                      <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-brand-primary/10 text-brand-primary">
                        <Bot size={20} />
                      </div>
                      <div className="min-w-0">
                        <h3 className="truncate font-semibold text-foreground">{agent.name}</h3>
                        <div className="mt-1 flex items-center gap-1.5">
                          <span className="text-xs text-content-muted capitalize">{agent.role}</span>
                          <span className="text-xs text-content-muted/60">·</span>
                          <span className={`inline-flex items-center gap-0.5 rounded px-1 py-0.2 text-[10px] font-bold uppercase ${agent.model_level_group === "fast"
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
                    <button
                      onClick={() => setConfirmingAgentID(agent.id)}
                      className="rounded-md p-1.5 text-content-muted opacity-0 transition hover:bg-danger/10 hover:text-danger group-hover:opacity-100 cursor-pointer"
                      title="Dismiss organization agent"
                      type="button"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>

                  <p className="line-clamp-3 text-sm text-content-muted">{agent.goal}</p>

                  <div className="mt-4 flex flex-wrap items-center gap-2">
                    <Badge value={agent.role} />
                    <Badge value={agent.status || "idle"} />
                    <Badge value={agent.autonomy_level} />
                    <span className="rounded border border-stroke bg-surface px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
                      {isAutoJoin ? "auto join" : "manual"}
                    </span>
                  </div>

                  <div className="mt-4 border-t border-stroke pt-3">
                    <h4 className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Project assignments</h4>
                    {isAutoJoin ? (
                      <p className="rounded border border-emerald-500/25 bg-emerald-500/10 p-2 text-xs text-emerald-600 dark:text-emerald-300 font-medium">Inherited by all projects</p>
                    ) : assignedProjectNames.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5">
                        {assignedProjectNames.map((projectName) => (
                          <span key={projectName} className="rounded border border-stroke bg-surface px-2 py-1 text-xs text-foreground font-medium">
                            {projectName}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs italic text-content-muted">Not assigned to any projects.</p>
                    )}
                  </div>

                  {agentActionErrors[agent.id] && (
                    <p className="mt-3 rounded border border-red-500/20 bg-red-500/10 p-2 text-xs text-red-600 dark:text-red-400 font-medium">{agentActionErrors[agent.id]}</p>
                  )}
                </div>

                {confirmingAgentID === agent.id && (
                  <div className="mt-4 rounded-md border border-red-500/20 bg-red-500/10 p-3">
                    <p className="text-xs text-red-600 dark:text-red-300 font-medium">Dismiss this organization agent?</p>
                    <div className="mt-3 flex gap-2">
                      <button onClick={() => removeAgent(agent.id)} className="rounded bg-danger px-3 py-1 text-xs font-semibold text-white hover:opacity-90 cursor-pointer" type="button">
                        Dismiss
                      </button>
                      <button onClick={() => setConfirmingAgentID("")} className="rounded border border-stroke px-3 py-1 text-xs font-semibold text-foreground hover:bg-surface cursor-pointer" type="button">
                        Cancel
                      </button>
                    </div>
                  </div>
                )}

                {!isAutoJoin && assignableProjects.length > 0 && (
                  <div className="mt-4 flex gap-2 border-t border-stroke pt-3">
                    <div className="relative flex-1">
                      <select
                        value={assigningMap[agent.id] || ""}
                        onChange={(e) => setAssigningMap((prev) => ({ ...prev, [agent.id]: e.target.value }))}
                        className="w-full appearance-none rounded border border-stroke bg-background pl-2 pr-8 py-1 text-xs text-foreground focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20 transition-all duration-150"
                      >
                        <option value="">Assign to project</option>
                        {assignableProjects.map((project) => (
                          <option key={project.id} value={project.id}>{project.name}</option>
                        ))}
                      </select>
                      <ChevronDown className="absolute right-2 top-2 text-content-muted pointer-events-none" size={11} />
                    </div>
                    <button
                      onClick={() => assignAgentToProject(agent)}
                      disabled={!assigningMap[agent.id]}
                      className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                      type="button"
                    >
                      Add
                      <ArrowRight size={12} />
                    </button>
                  </div>
                )}
              </article>
            );
          })}
        </div>
      )}

      {isHireModalOpen && (
        <HireAgentWizard
          roleTemplates={roleTemplates}
          isSubmitting={isSubmitting}
          error={formError}
          onClose={resetHireModal}
          onSubmit={handleHireAgent}
        />
      )}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <article className="glass-panel glow-on-hover rounded-lg p-4">
      <div className="font-mono text-lg font-semibold text-foreground">{value}</div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}

function AgentCardsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="glass-panel rounded-lg p-5">
          <div className="mb-3 flex items-start gap-3">
            <div className="skeleton-shimmer size-10 rounded-lg" />
            <div className="flex-1 space-y-2">
              <div className="skeleton-shimmer h-5 w-40 rounded" />
              <div className="skeleton-shimmer h-3 w-28 rounded" />
            </div>
          </div>
          <div className="skeleton-shimmer h-12 rounded" />
          <div className="mt-4 flex gap-2">
            <div className="skeleton-shimmer h-5 w-16 rounded" />
            <div className="skeleton-shimmer h-5 w-20 rounded" />
            <div className="skeleton-shimmer h-5 w-14 rounded" />
          </div>
          <div className="mt-4 border-t border-stroke pt-3">
            <div className="skeleton-shimmer h-8 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}

function providerSummary(agents: Agent[]) {
  const counts = agents.reduce<Record<string, number>>((acc, agent) => {
    const levelGroup = agent.model_level_group || "";
    const provider = levelGroup.includes("/") ? levelGroup.split("/")[0] : "gateway";
    acc[provider] = (acc[provider] || 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts).map(([provider, count]) => `${provider} (${count})`).join(", ");
}

function seedFleetTitle(agentCount: number) {
  if (agentCount >= DEFAULT_FLEET.length) return "Fleet already seeded";
  if (agentCount > 0) return `${agentCount} of ${DEFAULT_FLEET.length} agents created - click to retry`;
  return "Create the default seven-agent fleet";
}
