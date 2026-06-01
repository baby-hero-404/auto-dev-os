"use client";

import { FormEvent, useState } from "react";
import useSWR from "swr";
import {
  ArrowRight,
  Bot,
  Braces,
  Gauge,
  Layers3,
  Loader2,
  Plus,
  Route,
  ShieldAlert,
  Sparkles,
  Trash2,
  WandSparkles,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import type { Agent } from "@/lib/types";

const AGENT_ROLES = ["planner", "backend", "frontend", "reviewer", "qa"] as const;
const PROVIDERS = ["gateway", "openai", "anthropic", "gemini", "9router"] as const;
const LEVELS = ["easy", "medium", "hard"] as const;
const STRATEGIES = ["manual", "auto_join"] as const;
const AUTO_MODEL_VALUE = "__auto__";

const GATEWAY_MODEL_BY_LEVEL: Record<string, string> = {
  easy: "fast",
  medium: "balanced",
  hard: "powerful",
};

const MODEL_OPTIONS_BY_PROVIDER: Record<string, string[]> = {
  gateway: ["fast", "balanced", "powerful"],
  openai: ["gpt-4o-mini", "gpt-4o"],
  anthropic: ["claude-sonnet-4-20250514", "claude-opus-4-20250514"],
  gemini: ["gemini-2.5-flash", "gemini-2.5-pro"],
  "9router": ["balanced", "fast", "powerful", "premium-coding"],
};

const GATEWAY_FLOW = [
  "IDE/client request",
  "RTK tool-result compression",
  "Provider format translation",
  "Tiered fallback route",
  "Normalized response",
];

const TOKEN_SAVER_FILTERS = ["git diff", "grep", "find", "ls", "tree", "build logs"];

const FALLBACK_TIERS = [
  { name: "Tier 1", label: "Subscription", tone: "text-emerald-300", description: "Use paid quota first." },
  { name: "Tier 2", label: "Cheap", tone: "text-cyan-300", description: "Route to low-cost APIs." },
  { name: "Tier 3", label: "Free", tone: "text-amber-300", description: "Keep work unblocked." },
];

export default function AgentsPage() {
  const session = useSession();
  const [agentName, setAgentName] = useState("");
  const [agentRole, setAgentRole] = useState<string>(AGENT_ROLES[0]);
  const [agentProvider, setAgentProvider] = useState<string>(PROVIDERS[0]);
  const [agentModel, setAgentModel] = useState(AUTO_MODEL_VALUE);
  const [agentLevel, setAgentLevel] = useState<string>(LEVELS[0]);
  const [agentStrategy, setAgentStrategy] = useState<string>(STRATEGIES[0]);
  const [formError, setFormError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isSeeding, setIsSeeding] = useState(false);
  const [assigningMap, setAssigningMap] = useState<Record<string, string>>({}); // agentID -> selectedProjectID to assign

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  // Fetch all projects in organization
  const { data: projects = [] } = useSWR(
    session ? ["projects", orgID, token] : null,
    ([, oid, t]) => api.listProjects(oid, t),
  );

  // Fetch all organization staff pool agents
  const { data: orgAgents = [], mutate: mutateOrgAgents } = useSWR(
    session ? ["org-agents", orgID, token] : null,
    ([, oid, t]) => api.listOrgAgents(oid, t),
  );

  const providerCounts = orgAgents.reduce<Record<string, number>>((counts, agent) => {
    counts[agent.provider] = (counts[agent.provider] || 0) + 1;
    return counts;
  }, {});
  const gatewayAgents = providerCounts.gateway || 0;
  const manualAgents = orgAgents.filter((agent) => agent.assignment_strategy !== "auto_join").length;
  const autoJoinAgents = orgAgents.length - manualAgents;
  const modelOptions = MODEL_OPTIONS_BY_PROVIDER[agentProvider] || [];
  const resolvedModel = resolveAgentModel(agentProvider, agentModel, agentLevel);

  // Compile assignments: agentID -> array of project names they belong to
  const { data: assignments = {}, mutate: mutateAssignments } = useSWR(
    session && projects.length > 0 ? ["assignments", projects.map(p => p.id).join(","), token] : null,
    async ([, idsStr, t]) => {
      const ids = idsStr.split(",");
      const map: Record<string, string[]> = {};
      await Promise.all(
        ids.map(async (pid) => {
          try {
            const project = projects.find(p => p.id === pid);
            const list = await api.listAgents(pid, t);
            list.forEach((agent) => {
              if (!map[agent.id]) map[agent.id] = [];
              if (project && !map[agent.id].includes(project.name)) {
                map[agent.id].push(project.name);
              }
            });
          } catch (e) {
            console.error("Failed to fetch project agents", pid, e);
          }
        })
      );
      return map;
    }
  );

  // Hire new staff agent at Organization level
  async function handleHireAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !orgID) return;

    const name = agentName.trim();
    if (!name) {
      setFormError("Agent name is required.");
      return;
    }

    setFormError("");
    setIsSubmitting(true);
    try {
      await api.hireAgent(orgID, token, {
        name,
        role: agentRole,
        provider: agentProvider,
        model: resolvedModel,
        level: agentLevel,
        assignment_strategy: agentStrategy,
      });
      setAgentName("");
      mutateOrgAgents();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to hire agent");
    } finally {
      setIsSubmitting(false);
    }
  }

  // Seed default fleet of agents at Organization level
  async function seedDefaultFleet() {
    if (!token || !orgID || isSeeding) return;

    setIsSeeding(true);
    setFormError("");

    const fleet = [
      { name: "AI Planner", role: "planner", provider: "gateway", model: "fast", level: "easy", assignment_strategy: "auto_join" },
      { name: "AI Backend Developer", role: "backend", provider: "gateway", model: "balanced", level: "medium", assignment_strategy: "manual" },
      { name: "AI Frontend Developer", role: "frontend", provider: "gateway", model: "balanced", level: "medium", assignment_strategy: "manual" },
      { name: "AI Reviewer", role: "reviewer", provider: "gateway", model: "powerful", level: "hard", assignment_strategy: "manual" },
      { name: "AI QA Tester", role: "qa", provider: "gateway", model: "balanced", level: "medium", assignment_strategy: "manual" },
    ];

    try {
      for (const item of fleet) {
        await api.hireAgent(orgID, token, item);
      }
      mutateOrgAgents();
      mutateAssignments();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to seed default fleet");
    } finally {
      setIsSeeding(false);
    }
  }

  // Delete staff agent from Org pool
  async function removeAgent(agentID: string) {
    if (confirm("Are you sure you want to dismiss this agent from the Organization pool?")) {
      try {
        await api.deleteAgent(agentID, token);
        mutateOrgAgents();
        mutateAssignments();
      } catch (err) {
        alert(err instanceof ApiError ? err.message : "Failed to remove agent");
      }
    }
  }

  // Assign an existing manual staff agent to a project
  async function assignAgentToProject(agent: Agent) {
    const projectID = assigningMap[agent.id];
    if (!projectID || !token) return;

    try {
      await api.createAgent(projectID, token, {
        name: agent.name,
        role: agent.role,
        provider: agent.provider,
        model: agent.model,
        level: agent.level,
        assignment_strategy: agent.assignment_strategy,
        agent_id: agent.id, // passes original ID to assign
      });
      // clear dropdown selection for this agent
      setAssigningMap(prev => {
        const next = { ...prev };
        delete next[agent.id];
        return next;
      });
      // reload mappings
      mutateAssignments();
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Failed to assign agent to project");
    }
  }

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Agents Pool</h2>
          <p className="mt-1 text-sm text-[var(--muted)]">
            Manage your organization staff fleet with gateway-ready routing, token saving, and fallback tiers.
          </p>
        </div>
        <button
          onClick={seedDefaultFleet}
          disabled={isSeeding || orgAgents.length > 0}
          className="flex items-center justify-center gap-2 rounded-md border border-[var(--border)] bg-slate-900 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-slate-800 disabled:opacity-50 cursor-pointer"
          type="button"
        >
          {isSeeding ? (
            <>
              <Loader2 size={16} className="animate-spin" />
              Seeding fleet...
            </>
          ) : (
            <>
              <Sparkles size={16} className="text-[var(--accent)]" />
              Seed Default Fleet
            </>
          )}
        </button>
      </div>

      <section className="mb-6 overflow-hidden rounded-lg border border-[var(--border)] bg-[var(--primary)]">
        <div className="grid gap-0 lg:grid-cols-[1.15fr_0.85fr]">
          <div className="border-b border-[var(--border)] p-5 lg:border-b-0 lg:border-r">
            <div className="mb-4 flex items-center gap-3">
              <div className="grid size-10 place-items-center rounded-lg bg-[var(--accent)]/10 text-[var(--accent)]">
                <Route size={20} />
              </div>
              <div>
                <h3 className="font-mono font-semibold">Local AI Gateway Pattern</h3>
                <p className="text-sm text-[var(--muted)]">
                  Inspired by 9router: proxy requests before model execution to reduce payload cost and avoid provider lock-in.
                </p>
              </div>
            </div>
            <div className="grid gap-2 sm:grid-cols-5">
              {GATEWAY_FLOW.map((step, index) => (
                <div key={step} className="rounded-md border border-[var(--border)] bg-slate-950/45 p-3">
                  <div className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-[var(--accent)]">
                    {String(index + 1).padStart(2, "0")}
                  </div>
                  <p className="text-xs text-slate-200">{step}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="grid gap-4 p-5 sm:grid-cols-3 lg:grid-cols-1">
            <GatewayMetric icon={Gauge} label="Gateway agents" value={`${gatewayAgents}/${orgAgents.length}`} />
            <GatewayMetric icon={Layers3} label="Assignments" value={`${manualAgents} manual · ${autoJoinAgents} auto`} />
            <GatewayMetric icon={ShieldAlert} label="Resilience" value="quota/error fallback" />
          </div>
        </div>
      </section>

      <div className="mb-6 grid gap-4 lg:grid-cols-3">
        <article className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
          <div className="mb-3 flex items-center gap-2">
            <WandSparkles size={18} className="text-[var(--accent)]" />
            <h3 className="font-mono font-semibold">RTK Token Saver</h3>
          </div>
          <p className="text-sm text-[var(--muted)]">
            Detect long tool outputs before provider translation and compress structured noise without dropping intent.
          </p>
          <div className="mt-4 flex flex-wrap gap-2">
            {TOKEN_SAVER_FILTERS.map((filter) => (
              <span key={filter} className="rounded-full border border-[var(--border)] bg-slate-950/60 px-2.5 py-1 text-xs text-slate-200">
                {filter}
              </span>
            ))}
          </div>
        </article>

        <article className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
          <div className="mb-3 flex items-center gap-2">
            <Braces size={18} className="text-[var(--accent)]" />
            <h3 className="font-mono font-semibold">Format Translation</h3>
          </div>
          <p className="text-sm text-[var(--muted)]">
            Agents can target a single gateway model tier while the runtime translates OpenAI, Claude, Gemini, Cursor, and tool-call shapes.
          </p>
          <div className="mt-4 rounded-md border border-[var(--border)] bg-slate-950/60 p-3 font-mono text-xs text-slate-300">
            client payload → gateway normalizer → provider adapter
          </div>
        </article>

        <article className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
          <div className="mb-3 flex items-center gap-2">
            <Layers3 size={18} className="text-[var(--accent)]" />
            <h3 className="font-mono font-semibold">3-Tier Fallback</h3>
          </div>
          <div className="space-y-2">
            {FALLBACK_TIERS.map((tier) => (
              <div key={tier.name} className="rounded-md border border-[var(--border)] bg-slate-950/45 p-3">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-mono text-xs text-[var(--muted)]">{tier.name}</span>
                  <span className={`text-xs font-semibold ${tier.tone}`}>{tier.label}</span>
                </div>
                <p className="mt-1 text-xs text-slate-300">{tier.description}</p>
              </div>
            ))}
          </div>
        </article>
      </div>

      {/* Create staff agent form */}
      <div className="mb-6 rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
        <div className="mb-4 flex items-center gap-2">
          <Bot size={18} className="text-[var(--accent)]" />
          <h3 className="font-mono font-semibold">Hire New Agent</h3>
        </div>
        <form className="grid gap-4 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-7 items-end" onSubmit={handleHireAgent}>
          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-[var(--muted)] font-mono font-bold uppercase tracking-wider">Name</label>
            <input
              value={agentName}
              onChange={(e) => setAgentName(e.target.value)}
              placeholder="e.g. Coder Agent"
              className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)]"
              disabled={isSubmitting || isSeeding}
              required
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-[var(--muted)] font-mono font-bold uppercase tracking-wider">Role</label>
            <select
              value={agentRole}
              onChange={(e) => setAgentRole(e.target.value)}
              className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] cursor-pointer"
              disabled={isSubmitting || isSeeding}
            >
              {AGENT_ROLES.map((role) => (
                <option key={role} value={role}>
                  {role}
                </option>
              ))}
            </select>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-[var(--muted)] font-mono font-bold uppercase tracking-wider">Level</label>
            <select
              value={agentLevel}
              onChange={(e) => setAgentLevel(e.target.value)}
              className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] cursor-pointer"
              disabled={isSubmitting || isSeeding}
            >
              {LEVELS.map((lvl) => (
                <option key={lvl} value={lvl}>
                  {lvl}
                </option>
              ))}
            </select>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-[var(--muted)] font-mono font-bold uppercase tracking-wider">Provider</label>
            <select
              value={agentProvider}
              onChange={(e) => {
                setAgentProvider(e.target.value);
                setAgentModel(AUTO_MODEL_VALUE);
              }}
              className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] cursor-pointer"
              disabled={isSubmitting || isSeeding}
            >
              {PROVIDERS.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-[var(--muted)] font-mono font-bold uppercase tracking-wider">Model</label>
            <select
              value={agentModel}
              onChange={(e) => setAgentModel(e.target.value)}
              className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] cursor-pointer"
              disabled={isSubmitting || isSeeding}
            >
              <option value={AUTO_MODEL_VALUE}>Auto by level ({resolvedModel})</option>
              {modelOptions.map((model) => (
                <option key={model} value={model}>
                  {model}
                </option>
              ))}
            </select>
            <p className="text-[11px] text-[var(--muted)]">
              Prefer Auto for gateway agents; it stores a safe tier instead of a typo-prone raw model.
            </p>
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-xs text-[var(--muted)] font-mono font-bold uppercase tracking-wider">Strategy</label>
            <select
              value={agentStrategy}
              onChange={(e) => setAgentStrategy(e.target.value)}
              className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)] cursor-pointer"
              disabled={isSubmitting || isSeeding}
            >
              {STRATEGIES.map((strat) => (
                <option key={strat} value={strat}>
                  {strat.replace("_", " ")}
                </option>
              ))}
            </select>
          </div>

          <button
            className="flex items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer w-full disabled:opacity-50"
            disabled={isSubmitting || isSeeding}
            type="submit"
          >
            {isSubmitting ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Hiring...
              </>
            ) : (
              <>
                <Plus size={16} />
                Hire Agent
              </>
            )}
          </button>
        </form>
        {formError && (
          <p className="mt-3 rounded border border-red-500/20 bg-red-950/40 p-2 text-xs text-red-200">
            {formError}
          </p>
        )}
      </div>

      {/* Agents grid */}
      {orgAgents.length === 0 ? (
        <EmptyState
          icon={Bot}
          title="No agents hired yet"
          description="Create your first agent above, or seed a default fleet to build your platform staff instantly."
        />
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {orgAgents.map((agent: Agent) => {
            const assignedProjectNames = assignments[agent.id] || [];
            const isAutoJoin = agent.assignment_strategy === "auto_join";

            // Filter projects this agent is NOT currently assigned to
            const assignableProjects = projects.filter(
              (p) => !assignedProjectNames.includes(p.name)
            );

            return (
              <article
                key={agent.id}
                className="group rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]/30 flex flex-col justify-between"
              >
                <div>
                  <div className="mb-3 flex items-start justify-between">
                    <div className="flex items-center gap-3">
                      <div className="grid size-10 place-items-center rounded-lg bg-[var(--accent)]/10 text-[var(--accent)]">
                        <Bot size={20} />
                      </div>
                      <div>
                        <h3 className="font-mono font-semibold text-white">{agent.name}</h3>
                        <p className="text-xs text-[var(--muted)]">{agent.provider}/{agent.model}</p>
                      </div>
                    </div>
                    <button
                      onClick={() => removeAgent(agent.id)}
                      className="rounded-md p-1.5 text-slate-500 opacity-0 transition hover:bg-red-950/40 hover:text-red-300 group-hover:opacity-100 cursor-pointer"
                      title="Dismiss agent"
                      type="button"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>

                  <div className="flex flex-wrap items-center gap-2 mt-4">
                    <Badge value={agent.role} />
                    <Badge value={agent.status || "idle"} />
                    <Badge value={agent.level || "easy"} />
                    <span className="rounded border border-[var(--border)] px-2 py-0.5 text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)] bg-slate-900/60">
                      {isAutoJoin ? "auto join" : "manual"}
                    </span>
                  </div>

                  {/* Project Assignments */}
                  <div className="mt-4 border-t border-[var(--border)] pt-3">
                    <h4 className="text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)] mb-2">
                      Project Assignments
                    </h4>
                    {isAutoJoin ? (
                      <p className="text-xs text-emerald-300/90 bg-emerald-950/20 border border-emerald-500/20 rounded p-2 flex items-center gap-2">
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75"></span>
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500"></span>
                        </span>
                        Auto-assigned to all projects
                      </p>
                    ) : assignedProjectNames.length > 0 ? (
                      <div className="flex flex-wrap gap-1.5">
                        {assignedProjectNames.map((pName) => (
                          <span
                            key={pName}
                            className="rounded bg-slate-900 px-2 py-1 text-xs text-white border border-[var(--border)]"
                          >
                            {pName}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-[var(--muted)] italic">
                        Not assigned to any projects.
                      </p>
                    )}
                  </div>
                </div>

                {/* Add to Project action row (if manual strategy and projects exist) */}
                {!isAutoJoin && assignableProjects.length > 0 && (
                  <div className="mt-4 border-t border-[var(--border)] pt-3 flex gap-2">
                    <select
                      value={assigningMap[agent.id] || ""}
                      onChange={(e) =>
                        setAssigningMap((prev) => ({
                          ...prev,
                          [agent.id]: e.target.value,
                        }))
                      }
                      className="flex-1 rounded border border-[var(--border)] bg-slate-950 px-2 py-1 text-xs text-white focus:outline-none focus:border-[var(--accent)] cursor-pointer"
                    >
                      <option value="">— Assign to Project —</option>
                      {assignableProjects.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name}
                        </option>
                      ))}
                    </select>
                    <button
                      onClick={() => assignAgentToProject(agent)}
                      disabled={!assigningMap[agent.id]}
                      className="rounded bg-[var(--accent)] px-3 py-1 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 flex items-center gap-1 cursor-pointer"
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
    </DashboardLayout>
  );
}

function GatewayMetric({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof Gauge;
  label: string;
  value: string;
}) {
  return (
    <article className="rounded-lg border border-[var(--border)] bg-slate-950/45 p-4">
      <div className="mb-3 grid size-9 place-items-center rounded-md bg-[var(--accent)]/10 text-[var(--accent)]">
        <Icon size={18} />
      </div>
      <div className="font-mono text-lg font-semibold text-white">{value}</div>
      <div className="text-xs text-[var(--muted)]">{label}</div>
    </article>
  );
}

function resolveAgentModel(provider: string, selectedModel: string, level: string) {
  if (selectedModel !== AUTO_MODEL_VALUE) return selectedModel;
  if (provider === "gateway") return GATEWAY_MODEL_BY_LEVEL[level] || "balanced";
  return MODEL_OPTIONS_BY_PROVIDER[provider]?.[0] || "default";
}
