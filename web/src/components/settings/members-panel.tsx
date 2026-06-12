"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import { ArrowRight, Bot, Check, ChevronDown, Loader2, Plus, Sparkles, Trash2 } from "lucide-react";
import { Toaster, toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Agent, Skill } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { HireAgentWizard, type HireAgentPayload } from "@/components/dashboard/hire-agent-wizard";

const DEFAULT_FLEET = [
  {
    name: "AI Planner",
    role: "planner",
    goal: "Break requirements into execution plans, milestones, and task dependencies.",
    model_route: "fast",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Backend Developer",
    role: "backend",
    goal: "Implement server-side features, persistence, APIs, and integration logic.",
    model_route: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Frontend Developer",
    role: "frontend",
    goal: "Build user-facing flows, application state, and responsive interface behavior.",
    model_route: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI Reviewer",
    role: "reviewer",
    goal: "Review changes for correctness, regressions, security issues, and missing tests.",
    model_route: "powerful",
    autonomy_level: "approval_required",
    assignment_strategy: "auto_join",
  },
  {
    name: "AI QA Tester",
    role: "qa",
    goal: "Design and run verification checks for functional and regression coverage.",
    model_route: "balanced",
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
  const [savingSkillsAgentID, setSavingSkillsAgentID] = useState("");
  const [expandedSkillAgentIDs, setExpandedSkillAgentIDs] = useState<Record<string, boolean>>({});
  const [draftSkillIDsByAgent, setDraftSkillIDsByAgent] = useState<Record<string, string[]>>({});
  const [savedSkillsAgentID, setSavedSkillsAgentID] = useState("");

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

  const { data: skills = [], isLoading: isSkillsLoading } = useAuthedSWR(
    token ? ["skills"] : null,
    (t) => api.listSkills(t),
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

  const agentSkillsKey = useMemo(() => {
    if (!session || orgAgents.length === 0) return null;
    return ["org-agent-skills", orgAgents.map((agent) => agent.id).sort().join(",")] as const;
  }, [orgAgents, session]);

  const { data: agentSkills = {}, mutate: mutateAgentSkills, isLoading: isAgentSkillsLoading } = useSWR(
    agentSkillsKey,
    async ([, idsStr]) => {
      const map: Record<string, Skill[]> = {};
      await Promise.all(
        idsStr.split(",").filter(Boolean).map(async (agentID) => {
          try {
            map[agentID] = await api.listAgentSkills(agentID, token);
          } catch (err) {
            console.error("Failed to fetch agent skills", agentID, err);
            map[agentID] = [];
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
      mutateAgentSkills();
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
        model_route: agent.model_route,
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

    const roleSkillNames: Record<string, string[]> = {
      planner: ["plan-writing", "brainstorming", "context-management"],
      backend: ["clean-code", "api-patterns", "database-design", "nodejs-best-practices", "python-patterns", "golang-best-practices"],
      frontend: ["clean-code", "nextjs-best-practices", "react-patterns", "tailwind-patterns", "typescript-expert", "ux-ui-pro-max"],
      reviewer: ["clean-code", "code-review-checklist", "review-pre-commit-git"],
      qa: ["testing-patterns", "tdd-workflow", "webapp-testing"],
    };

    try {
      // 1. If skills are empty, seed them first!
      let currentSkills = skills;
      if (currentSkills.length === 0) {
        try {
          toast.info("Seeding default skills first...");
          currentSkills = await api.seedSkills(token);
        } catch (err) {
          console.error("Failed to seed default skills automatically", err);
        }
      }

      for (const agent of DEFAULT_FLEET) {
        if (existingNames.has(agent.name)) continue;
        try {
          const targetSkillNames = roleSkillNames[agent.role] || [];
          const skillIDs = currentSkills
            .filter((s) => targetSkillNames.includes(s.name))
            .map((s) => s.id);

          await api.hireAgent(orgID, token, {
            ...agent,
            skill_ids: skillIDs,
          });
          created += 1;
          toast.success(`${agent.name} created with ${skillIDs.length} skills.`);
        } catch (err) {
          const message = err instanceof ApiError ? err.message : "Failed to create agent.";
          toast.error(`${agent.name}: ${message}`);
        }
      }
      await mutateOrgAgents();
      await mutateAgentSkills();
      await mutateAssignments();
      if (created === 0) {
        toast.info("No new default agents were created.");
      }
    } finally {
      setIsSeeding(false);
    }
  }
  function toggleSkillsPanel(agent: Agent) {
    setExpandedSkillAgentIDs((prev) => ({ ...prev, [agent.id]: !prev[agent.id] }));
    setDraftSkillIDsByAgent((prev) => {
      if (prev[agent.id]) return prev;
      if (!agentSkills[agent.id]) return prev;
      return {
        ...prev,
        [agent.id]: (agentSkills[agent.id] || []).map((skill) => skill.id),
      };
    });
  }

  function toggleDraftSkill(agentID: string, skillID: string, fallbackIDs: string[]) {
    setDraftSkillIDsByAgent((prev) => {
      const current = prev[agentID] ?? fallbackIDs;
      return {
        ...prev,
        [agentID]: current.includes(skillID)
          ? current.filter((id) => id !== skillID)
          : [...current, skillID],
      };
    });
  }

  async function saveAgentSkills(agent: Agent) {
    if (!token || savingSkillsAgentID) return;
    const assignedIDs = (agentSkills[agent.id] || []).map((skill) => skill.id);
    const nextSkillIDs = draftSkillIDsByAgent[agent.id] ?? assignedIDs;

    setSavingSkillsAgentID(agent.id);
    setAgentActionErrors((prev) => ({ ...prev, [agent.id]: "" }));
    try {
      await api.bulkReplaceAgentSkills(agent.id, nextSkillIDs, token);
      mutateAgentSkills({
        ...agentSkills,
        [agent.id]: skills.filter((skill) => nextSkillIDs.includes(skill.id)),
      }, false);
      setSavedSkillsAgentID(agent.id);
      window.setTimeout(() => {
        setSavedSkillsAgentID((current) => (current === agent.id ? "" : current));
      }, 2000);
    } catch (err) {
      setAgentActionErrors((prev) => ({
        ...prev,
        [agent.id]: err instanceof ApiError ? err.message : "Failed to update skills",
      }));
    } finally {
      setSavingSkillsAgentID("");
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
            const assignedSkillIDs = (agentSkills[agent.id] || []).map((skill) => skill.id);
            const assignedTools = toolsForSkills(agentSkills[agent.id] || []);
            const draftSkillIDs = draftSkillIDsByAgent[agent.id] ?? assignedSkillIDs;
            const isSkillsExpanded = Boolean(expandedSkillAgentIDs[agent.id]);
            const isAgentSkillLoading = isAgentSkillsLoading && !agentSkills[agent.id];
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
                        <p className="truncate text-xs text-content-muted">
                          {agent.role} · {agent.model_route} · {assignedSkillIDs.length} {assignedSkillIDs.length === 1 ? "skill" : "skills"} · {assignedTools.length} {assignedTools.length === 1 ? "tool" : "tools"}
                        </p>
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
                    <button
                      onClick={() => toggleSkillsPanel(agent)}
                      className="flex w-full items-center justify-between gap-3 rounded-md px-2 py-1.5 text-left transition hover:bg-surface cursor-pointer"
                      type="button"
                    >
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Skills</span>
                        <span className="rounded border border-stroke bg-surface px-2 py-0.5 text-[11px] font-semibold text-foreground">
                          {assignedSkillIDs.length} {assignedSkillIDs.length === 1 ? "skill" : "skills"}
                        </span>
                      </span>
                      <span className="flex shrink-0 items-center gap-2 text-xs text-content-muted">
                        {isAgentSkillsLoading && !agentSkills[agent.id] && <Loader2 size={13} className="animate-spin" />}
                        <ChevronDown size={15} className={`transition ${isSkillsExpanded ? "rotate-180" : ""}`} />
                      </span>
                    </button>
                    {isSkillsExpanded && (
                      <AgentSkillsPanel
                        skills={skills}
                        selectedIDs={draftSkillIDs}
                        isSkillsLoading={isSkillsLoading}
                        isAssignedLoading={isAgentSkillLoading}
                        isSaving={savingSkillsAgentID === agent.id}
                        isSaved={savedSkillsAgentID === agent.id}
                        onToggle={(skillID) => toggleDraftSkill(agent.id, skillID, assignedSkillIDs)}
                        onSave={() => saveAgentSkills(agent)}
                        tools={assignedTools}
                      />
                    )}
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
          skills={skills}
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
    const route = agent.model_route || "";
    const provider = route.includes("/") ? route.split("/")[0] : "gateway";
    acc[provider] = (acc[provider] || 0) + 1;
    return acc;
  }, {});
  return Object.entries(counts).map(([provider, count]) => `${provider} (${count})`).join(", ");
}



function AgentSkillsPanel({
  skills,
  selectedIDs,
  isSkillsLoading,
  isAssignedLoading,
  isSaving,
  isSaved,
  onToggle,
  onSave,
  tools,
}: {
  skills: Skill[];
  selectedIDs: string[];
  isSkillsLoading: boolean;
  isAssignedLoading: boolean;
  isSaving: boolean;
  isSaved: boolean;
  onToggle: (skillID: string) => void;
  onSave: () => void;
  tools: string[];
}) {
  if (isSkillsLoading) {
    return <SkillsPanelSkeleton />;
  }

  const selected = new Set(selectedIDs);

  return (
    <div className="mt-3 rounded-md border border-stroke bg-background p-3">
      {skills.length === 0 ? (
        <p className="rounded border border-stroke bg-surface px-3 py-2 text-xs italic text-content-muted">No skills available.</p>
      ) : (
        <div className="grid max-h-48 gap-2 overflow-y-auto pr-1 sm:grid-cols-2">
          {skills.map((skill) => (
            <label
              key={skill.id}
              className={`flex min-w-0 items-center gap-2 rounded-md border px-2.5 py-2 text-xs transition cursor-pointer ${skillToneClass(skill)} ${selected.has(skill.id) ? "ring-1 ring-brand-primary/30" : ""}`}
              title={skill.description || skill.name}
            >
              <input
                type="checkbox"
                checked={selected.has(skill.id)}
                onChange={() => onToggle(skill.id)}
                disabled={isSaving || isAssignedLoading}
                className="size-3.5 shrink-0 accent-brand-primary cursor-pointer"
              />
              <span className="truncate font-mono font-semibold">{skill.name}</span>
            </label>
          ))}
        </div>
      )}
      <div className="mt-3 rounded border border-stroke bg-surface px-3 py-2">
        <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Tools</span>
        {tools.length === 0 ? (
          <p className="mt-1 text-xs italic text-content-muted">No tools assigned.</p>
        ) : (
          <div className="mt-2 flex flex-wrap gap-1.5">
            {tools.map((tool) => (
              <span key={tool} className="rounded border border-stroke bg-background px-2 py-0.5 font-mono text-[10px] font-semibold text-foreground">
                {tool}
              </span>
            ))}
          </div>
        )}
      </div>
      <div className="mt-3 flex items-center justify-between gap-3">
        <span className="inline-flex items-center gap-1 text-xs text-content-muted">
          {isAssignedLoading && <Loader2 size={12} className="animate-spin" />}
          {selected.size} selected
        </span>
        <div className="flex items-center gap-2">
          {isSaved && (
            <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
              <Check size={13} />
              Saved
            </span>
          )}
          <button
            onClick={onSave}
            disabled={isSaving || isAssignedLoading}
            className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-60 cursor-pointer"
            type="button"
          >
            {isSaving && <Loader2 size={13} className="animate-spin" />}
            Save Skills
          </button>
        </div>
      </div>
    </div>
  );
}

function SkillsPanelSkeleton() {
  return (
    <div className="mt-3 rounded-md border border-stroke bg-background p-3">
      <div className="grid gap-2 sm:grid-cols-2">
        {[0, 1, 2, 3].map((item) => (
          <div key={item} className="skeleton-shimmer h-9 rounded-md" />
        ))}
      </div>
      <div className="mt-3 flex justify-end">
        <div className="skeleton-shimmer h-8 w-24 rounded-md" />
      </div>
    </div>
  );
}

type SkillCategory = "file" | "git" | "test" | "security" | "database" | "other";

function skillCategory(skill: Skill): SkillCategory {
  const name = skill.name.toLowerCase();
  if (name.includes("file") || name.includes("read") || name.includes("write")) return "file";
  if (name.includes("git") || name.includes("commit") || name.includes("push")) return "git";
  if (name.includes("test") || name.includes("qa")) return "test";
  if (name.includes("security") || name.includes("scan") || name.includes("vuln")) return "security";
  if (name.includes("migration") || name.includes("db") || name.includes("schema")) return "database";
  return "other";
}

function skillToneClass(skill: Skill) {
  const tones: Record<SkillCategory, string> = {
    file: "border-blue-500/25 bg-blue-500/10 text-blue-700 hover:bg-blue-500/15 dark:text-blue-200",
    git: "border-emerald-500/25 bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/15 dark:text-emerald-200",
    test: "border-amber-500/25 bg-amber-500/10 text-amber-700 hover:bg-amber-500/15 dark:text-amber-200",
    security: "border-rose-500/25 bg-rose-500/10 text-rose-700 hover:bg-rose-500/15 dark:text-rose-200",
    database: "border-purple-500/25 bg-purple-500/10 text-purple-700 hover:bg-purple-500/15 dark:text-purple-200",
    other: "border-slate-500/20 bg-slate-500/10 text-slate-700 hover:bg-slate-500/15 dark:text-slate-200",
  };
  return tones[skillCategory(skill)];
}

function toolsForSkills(skills: Skill[]) {
  const tools = new Set<string>();
  for (const skill of skills) {
    addToolName(tools, skill.name);
    const schema = skill.schema;
    if (schema && typeof schema === "object" && !Array.isArray(schema)) {
      addSchemaTools(tools, schema.tool);
      addSchemaTools(tools, schema.tools);
      addSchemaTools(tools, schema.default_tools);
      addSchemaTools(tools, schema.allowed_tools);
      if (typeof schema.category === "string") {
        addCategoryTools(tools, schema.category);
      }
    }
  }
  return Array.from(tools).sort();
}

function addSchemaTools(tools: Set<string>, value: unknown) {
  if (typeof value === "string") {
    addToolName(tools, value);
    return;
  }
  if (Array.isArray(value)) {
    value.forEach((item) => {
      if (typeof item === "string") addToolName(tools, item);
    });
  }
}

function addToolName(tools: Set<string>, name: string) {
  const normalized = name.trim().toLowerCase().replaceAll("-", "_").replaceAll(" ", "_");
  if (["read_file", "write_file", "run_tests", "analyze_logs", "generate_docs", "create_migration", "search_code", "apply_patch"].includes(normalized)) {
    tools.add(normalized);
  }
}

function addCategoryTools(tools: Set<string>, category: string) {
  switch (category.trim().toLowerCase()) {
    case "test":
    case "testing":
    case "qa":
      tools.add("run_tests");
      tools.add("analyze_logs");
      break;
    case "database":
    case "db":
    case "migration":
      tools.add("create_migration");
      tools.add("read_file");
      tools.add("write_file");
      break;
    case "docs":
    case "documentation":
      tools.add("generate_docs");
      tools.add("read_file");
      break;
    case "code":
    case "file":
    case "git":
      tools.add("read_file");
      tools.add("write_file");
      tools.add("search_code");
      tools.add("apply_patch");
      break;
  }
}

function seedFleetTitle(agentCount: number) {
  if (agentCount >= DEFAULT_FLEET.length) return "Fleet already seeded";
  if (agentCount > 0) return `${agentCount} of ${DEFAULT_FLEET.length} agents created - click to retry`;
  return "Create the default five-agent fleet";
}
