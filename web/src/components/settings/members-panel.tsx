"use client";

import { FormEvent, useMemo, useState } from "react";
import useSWR from "swr";
import { ArrowRight, Bot, Loader2, Plus, Sparkles, Trash2, X } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Agent, RoleTemplate, Skill } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import { AGENT_ROLES, ASSIGNMENT_STRATEGIES, AUTONOMY_LEVELS, MODEL_ROUTES } from "@/lib/model-options";

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
    assignment_strategy: "manual",
  },
  {
    name: "AI Frontend Developer",
    role: "frontend",
    goal: "Build user-facing flows, application state, and responsive interface behavior.",
    model_route: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "manual",
  },
  {
    name: "AI Reviewer",
    role: "reviewer",
    goal: "Review changes for correctness, regressions, security issues, and missing tests.",
    model_route: "powerful",
    autonomy_level: "approval_required",
    assignment_strategy: "manual",
  },
  {
    name: "AI QA Tester",
    role: "qa",
    goal: "Design and run verification checks for functional and regression coverage.",
    model_route: "balanced",
    autonomy_level: "supervised",
    assignment_strategy: "manual",
  },
];

export function MembersPanel() {
  const session = useSession();
  const [isHireModalOpen, setIsHireModalOpen] = useState(false);
  const [agentName, setAgentName] = useState("");
  const [agentRole, setAgentRole] = useState<string>(AGENT_ROLES[0]);
  const [agentGoal, setAgentGoal] = useState("");
  const [agentAutonomy, setAgentAutonomy] = useState<string>(AUTONOMY_LEVELS[1]);
  const [agentModelRoute, setAgentModelRoute] = useState<string>("balanced");
  const [agentStrategy, setAgentStrategy] = useState<string>(ASSIGNMENT_STRATEGIES[0]);
  const [selectedSkillIDs, setSelectedSkillIDs] = useState<string[]>([]);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSeeding, setIsSeeding] = useState(false);
  const [assigningMap, setAssigningMap] = useState<Record<string, string>>({});
  const [confirmingAgentID, setConfirmingAgentID] = useState("");
  const [agentActionErrors, setAgentActionErrors] = useState<Record<string, string>>({});
  const [savingSkillsAgentID, setSavingSkillsAgentID] = useState("");

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: projects = [] } = useAuthedSWR(
    orgID ? ["projects", orgID] : null,
    (t) => api.listProjects(orgID, t),
  );

  const { data: orgAgents = [], mutate: mutateOrgAgents } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  const { data: roleTemplates = [] } = useAuthedSWR(
    token ? ["role-templates"] : null,
    (t) => api.listRoleTemplates(t),
  );

  const { data: skills = [] } = useAuthedSWR(
    token ? ["skills"] : null,
    (t) => api.listSkills(t),
  );

  const { data: modelRoutes = [] } = useAuthedSWR(
    orgID ? ["model-routes", orgID] : null,
    (t) => api.listModelRoutes(orgID, t),
  );

  const modelRouteOptions = useMemo(
    () => [...new Set([...MODEL_ROUTES, ...modelRoutes.map((route) => route.name), agentModelRoute].filter(Boolean))],
    [agentModelRoute, modelRoutes],
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

  const { data: agentSkills = {}, mutate: mutateAgentSkills } = useSWR(
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

  const roleCounts = orgAgents.reduce<Record<string, number>>((counts, agent) => {
    counts[agent.role] = (counts[agent.role] || 0) + 1;
    return counts;
  }, {});
  const manualAgents = orgAgents.filter((agent) => agent.assignment_strategy !== "auto_join").length;
  const autoJoinAgents = orgAgents.length - manualAgents;

  function templateForRole(role: string): RoleTemplate | undefined {
    return roleTemplates.find((template) => template.role === role);
  }

  function skillIDsForTemplate(template?: RoleTemplate): string[] {
    if (!template) return [];
    const defaultToolNames = new Set(template.default_tools);
    return skills.filter((skill) => defaultToolNames.has(skill.name)).map((skill) => skill.id);
  }

  function applyRole(role: string) {
    setAgentRole(role);
    const template = templateForRole(role);
    if (template && !agentGoal.trim()) {
      setAgentGoal(template.default_goal);
    }
    if (template) {
      setSelectedSkillIDs(skillIDsForTemplate(template));
    }
  }

  function resetHireModal() {
    setIsHireModalOpen(false);
    setAgentName("");
    setAgentRole(AGENT_ROLES[0]);
    setAgentGoal("");
    setAgentAutonomy(AUTONOMY_LEVELS[1]);
    setAgentModelRoute("balanced");
    setAgentStrategy(ASSIGNMENT_STRATEGIES[0]);
    setSelectedSkillIDs([]);
    setFormError("");
  }

  function toggleSelectedSkill(skillID: string) {
    setSelectedSkillIDs((current) => (
      current.includes(skillID) ? current.filter((id) => id !== skillID) : [...current, skillID]
    ));
  }

  async function handleHireAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;
    const name = agentName.trim();
    const goal = agentGoal.trim();
    if (!name) {
      setFormError("Agent name is required.");
      return;
    }
    if (!agentRole.trim()) {
      setFormError("Role is required.");
      return;
    }
    if (!goal) {
      setFormError("Goal is required.");
      return;
    }

    setFormError("");
    setIsSubmitting(true);
    try {
      await api.hireAgent(orgID, token, {
        name,
        role: agentRole,
        goal,
        autonomy_level: agentAutonomy,
        model_route: agentModelRoute,
        assignment_strategy: agentStrategy,
        skill_ids: selectedSkillIDs,
      });
      resetHireModal();
      mutateOrgAgents();
      mutateAgentSkills();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to hire agent");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function seedDefaultFleet() {
    if (!token || !orgID || isSeeding || orgAgents.length > 0) return;
    setIsSeeding(true);
    setFormError("");
    try {
      await Promise.all(DEFAULT_FLEET.map((agent) => api.hireAgent(orgID, token, agent)));
      mutateOrgAgents();
      mutateAssignments();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to seed default fleet");
    } finally {
      setIsSeeding(false);
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

  async function toggleAgentSkill(agent: Agent, skillID: string) {
    if (!token || savingSkillsAgentID) return;
    const currentSkills = agentSkills[agent.id] || [];
    const currentIDs = currentSkills.map((skill) => skill.id);
    const nextSkillIDs = currentIDs.includes(skillID)
      ? currentIDs.filter((id) => id !== skillID)
      : [...currentIDs, skillID];

    setSavingSkillsAgentID(agent.id);
    setAgentActionErrors((prev) => ({ ...prev, [agent.id]: "" }));
    try {
      await api.updateAgent(agent.id, token, { skill_ids: nextSkillIDs });
      mutateAgentSkills({
        ...agentSkills,
        [agent.id]: skills.filter((skill) => nextSkillIDs.includes(skill.id)),
      }, false);
    } catch (err) {
      setAgentActionErrors((prev) => ({
        ...prev,
        [agent.id]: err instanceof ApiError ? err.message : "Failed to update tools",
      }));
    } finally {
      setSavingSkillsAgentID("");
    }
  }

  return (
    <div className="space-y-6">
      <div className="grid gap-3 md:grid-cols-4">
        <Metric label="Total agents" value={orgAgents.length.toString()} />
        <Metric label="Auto-join" value={autoJoinAgents.toString()} />
        <Metric label="Manual" value={manualAgents.toString()} />
        <Metric
          label="Roles"
          value={
            Object.entries(roleCounts)
              .map(([role, count]) => `${role} ${count}`)
              .join(" / ") || "none"
          }
        />
      </div>

      <section className="rounded-lg border border-stroke bg-panel p-5">
        <div className="mb-4 flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
          <div className="flex items-center gap-2">
            <Bot size={18} className="text-brand-primary" />
            <h3 className="font-mono font-semibold">Agent pool actions</h3>
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              onClick={() => setIsHireModalOpen(true)}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90"
              type="button"
            >
              <Plus size={16} />
              Hire Agent
            </button>
            <button
              onClick={seedDefaultFleet}
              disabled={isSeeding || orgAgents.length > 0}
              className="inline-flex items-center justify-center gap-2 rounded-md border border-stroke bg-surface px-3 py-2 text-sm font-semibold text-white transition hover:bg-panel disabled:opacity-50"
              type="button"
            >
              {isSeeding ? <Loader2 size={16} className="animate-spin" /> : <Sparkles size={16} className="text-brand-primary" />}
              Seed Default Fleet
            </button>
          </div>
        </div>
        {formError && <p className="mt-3 rounded border border-red-500/20 bg-red-950/40 p-2 text-xs text-red-200">{formError}</p>}
      </section>

      {orgAgents.length === 0 ? (
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
                        <h3 className="truncate font-mono font-semibold text-white">{agent.name}</h3>
                        <p className="truncate text-xs text-content-muted">{agent.role} · {agent.model_route}</p>
                      </div>
                    </div>
                    <button
                      onClick={() => setConfirmingAgentID(agent.id)}
                      className="rounded-md p-1.5 text-slate-500 opacity-0 transition hover:bg-red-950/40 hover:text-red-300 group-hover:opacity-100"
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
                    <span className="rounded border border-stroke bg-panel/60 px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
                      {isAutoJoin ? "auto join" : "manual"}
                    </span>
                  </div>

                  <div className="mt-4 border-t border-stroke pt-3">
                    <div className="mb-2 flex items-center justify-between gap-2">
                      <h4 className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Allowed tools</h4>
                      {savingSkillsAgentID === agent.id && <Loader2 size={13} className="animate-spin text-content-muted" />}
                    </div>
                    <SkillSelector
                      skills={skills}
                      selectedIDs={(agentSkills[agent.id] || []).map((skill) => skill.id)}
                      onToggle={(skillID) => toggleAgentSkill(agent, skillID)}
                      disabled={savingSkillsAgentID !== "" || skills.length === 0}
                      emptyText="No tools available."
                    />
                  </div>

                  <div className="mt-4 border-t border-stroke pt-3">
                    <h4 className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">Project assignments</h4>
                    {isAutoJoin ? (
                      <p className="rounded border border-emerald-500/20 bg-emerald-950/20 p-2 text-xs text-emerald-300/90">Inherited by all projects</p>
                    ) : assignedProjectNames.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5">
                        {assignedProjectNames.map((projectName) => (
                          <span key={projectName} className="rounded border border-stroke bg-panel px-2 py-1 text-xs text-white">
                            {projectName}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs italic text-content-muted">Not assigned to any projects.</p>
                    )}
                  </div>

                  {agentActionErrors[agent.id] && (
                    <p className="mt-3 rounded border border-red-500/20 bg-red-950/40 p-2 text-xs text-red-200">{agentActionErrors[agent.id]}</p>
                  )}
                </div>

                {confirmingAgentID === agent.id && (
                  <div className="mt-4 rounded-md border border-red-500/20 bg-red-950/30 p-3">
                    <p className="text-xs text-red-100">Dismiss this organization agent?</p>
                    <div className="mt-3 flex gap-2">
                      <button onClick={() => removeAgent(agent.id)} className="rounded bg-red-500 px-3 py-1 text-xs font-semibold text-white" type="button">
                        Dismiss
                      </button>
                      <button onClick={() => setConfirmingAgentID("")} className="rounded border border-stroke px-3 py-1 text-xs font-semibold text-white" type="button">
                        Cancel
                      </button>
                    </div>
                  </div>
                )}

                {!isAutoJoin && assignableProjects.length > 0 && (
                  <div className="mt-4 flex gap-2 border-t border-stroke pt-3">
                    <select
                      value={assigningMap[agent.id] || ""}
                      onChange={(e) => setAssigningMap((prev) => ({ ...prev, [agent.id]: e.target.value }))}
                      className="min-w-0 flex-1 rounded border border-stroke bg-page px-2 py-1 text-xs text-white focus:border-brand-primary focus:outline-none"
                    >
                      <option value="">Assign to project</option>
                      {assignableProjects.map((project) => (
                        <option key={project.id} value={project.id}>{project.name}</option>
                      ))}
                    </select>
                    <button
                      onClick={() => assignAgentToProject(agent)}
                      disabled={!assigningMap[agent.id]}
                      className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50"
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
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-page/80 backdrop-blur-sm" onClick={() => !isSubmitting && resetHireModal()} />
          <div className="glass-panel relative w-full max-w-2xl rounded-lg p-5 shadow-2xl">
            <div className="mb-5 flex items-center justify-between border-b border-stroke pb-4">
              <div>
                <h3 className="font-mono text-lg font-semibold text-white">Hire Capability Agent</h3>
                <p className="text-sm text-content-muted">Configure role, goal, autonomy, tools, and gateway route.</p>
              </div>
              <button onClick={resetHireModal} disabled={isSubmitting} className="rounded-md p-1.5 text-content-muted transition hover:bg-panel hover:text-white" type="button">
                <X size={18} />
              </button>
            </div>

            <form onSubmit={handleHireAgent} className="space-y-4">
              <div className="grid gap-4 md:grid-cols-2">
                <Field label="Name">
                  <input
                    value={agentName}
                    onChange={(e) => setAgentName(e.target.value)}
                    placeholder="e.g. Backend Specialist"
                    className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                    disabled={isSubmitting}
                  />
                </Field>
                <Field label="Role">
                  <select
                    value={agentRole}
                    onChange={(e) => applyRole(e.target.value)}
                    className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                    disabled={isSubmitting}
                  >
                    {[...new Set([...AGENT_ROLES, ...roleTemplates.map((template) => template.role)])].map((role) => (
                      <option key={role} value={role}>{role}</option>
                    ))}
                  </select>
                </Field>
                <Field label="Autonomy">
                  <Select value={agentAutonomy} onChange={setAgentAutonomy} options={[...AUTONOMY_LEVELS]} disabled={isSubmitting} />
                </Field>
                <Field label="Model Route">
                  <Select value={agentModelRoute} onChange={setAgentModelRoute} options={modelRouteOptions} disabled={isSubmitting} />
                </Field>
                <Field label="Strategy">
                  <Select value={agentStrategy} onChange={setAgentStrategy} options={[...ASSIGNMENT_STRATEGIES]} disabled={isSubmitting} />
                </Field>
                <Field label="Custom Role">
                  <input
                    value={agentRole}
                    onChange={(e) => setAgentRole(e.target.value)}
                    className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                    disabled={isSubmitting}
                  />
                </Field>
              </div>
              <Field label="Goal">
                <textarea
                  value={agentGoal}
                  onChange={(e) => setAgentGoal(e.target.value)}
                  rows={4}
                  placeholder={templateForRole(agentRole)?.default_goal || "Describe what this agent is responsible for."}
                  className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                  disabled={isSubmitting}
                />
              </Field>
              <Field label="Allowed Tools">
                <SkillSelector
                  skills={skills}
                  selectedIDs={selectedSkillIDs}
                  onToggle={toggleSelectedSkill}
                  disabled={isSubmitting || skills.length === 0}
                  emptyText="No skills created yet."
                />
              </Field>

              {formError && <p className="rounded border border-red-500/20 bg-red-950/40 p-2 text-xs text-red-200">{formError}</p>}

              <div className="flex justify-end gap-2 border-t border-stroke pt-4">
                <button onClick={resetHireModal} className="rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-white" disabled={isSubmitting} type="button">
                  Cancel
                </button>
                <button className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 disabled:opacity-50" disabled={isSubmitting} type="submit">
                  {isSubmitting ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
                  Hire Agent
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <article className="glass-panel glow-on-hover rounded-lg p-4">
      <div className="font-mono text-lg font-semibold text-white">{value}</div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex min-w-0 flex-col gap-1.5">
      <span className="font-mono text-xs font-bold uppercase tracking-wider text-content-muted">{label}</span>
      {children}
    </label>
  );
}

function Select({
  value,
  onChange,
  options,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  options: string[];
  disabled?: boolean;
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
      disabled={disabled}
    >
      {options.map((option) => (
        <option key={option} value={option}>{option.replace("_", " ")}</option>
      ))}
    </select>
  );
}

function SkillSelector({
  skills,
  selectedIDs,
  onToggle,
  disabled,
  emptyText,
}: {
  skills: Skill[];
  selectedIDs: string[];
  onToggle: (skillID: string) => void;
  disabled?: boolean;
  emptyText: string;
}) {
  if (skills.length === 0) {
    return <p className="rounded border border-stroke bg-page px-3 py-2 text-xs italic text-content-muted">{emptyText}</p>;
  }
  const selected = new Set(selectedIDs);
  return (
    <div className="max-h-36 overflow-y-auto rounded-md border border-stroke bg-page p-2">
      <div className="grid gap-1.5 sm:grid-cols-2">
        {skills.map((skill) => (
          <label
            key={skill.id}
            className="flex min-w-0 items-center gap-2 rounded px-2 py-1.5 text-xs text-slate-200 transition hover:bg-panel"
            title={skill.description || skill.name}
          >
            <input
              type="checkbox"
              checked={selected.has(skill.id)}
              onChange={() => onToggle(skill.id)}
              disabled={disabled}
              className="size-3.5 shrink-0 accent-brand-primary"
            />
            <span className="truncate font-mono">{skill.name}</span>
          </label>
        ))}
      </div>
    </div>
  );
}
