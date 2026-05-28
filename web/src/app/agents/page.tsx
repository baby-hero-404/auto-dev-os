"use client";

import { FormEvent, useState } from "react";
import useSWR from "swr";
import { Bot, Plus, Trash2 } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { EmptyState } from "@/components/ui/empty-state";
import type { Agent, Project } from "@/lib/types";

const AGENT_ROLES = ["planner", "backend", "frontend", "reviewer", "qa"] as const;
const PROVIDERS = ["openai", "anthropic", "google"] as const;

export default function AgentsPage() {
  const session = useSession();
  const [selectedProject, setSelectedProject] = useState("");
  const [agentName, setAgentName] = useState("");
  const [agentRole, setAgentRole] = useState<string>(AGENT_ROLES[0]);
  const [agentProvider, setAgentProvider] = useState<string>(PROVIDERS[0]);
  const [agentModel, setAgentModel] = useState("gpt-4o");
  const [formError, setFormError] = useState("");

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: projects = [] } = useSWR(
    session ? ["projects", orgID, token] : null,
    ([, oid, t]) => api.listProjects(oid, t),
  );

  const { data: agents = [], mutate } = useSWR(
    selectedProject && token ? ["agents", selectedProject, token] : null,
    ([, pid, t]) => api.listAgents(pid, t),
  );

  async function createAgent(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedProject || !token) return;

    const name = agentName.trim();
    if (!name) {
      setFormError("Agent name is required.");
      return;
    }

    setFormError("");
    try {
      await api.createAgent(selectedProject, token, {
        name,
        role: agentRole,
        provider: agentProvider,
        model: agentModel.trim() || "gpt-4o",
      });
      setAgentName("");
      mutate();
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : "Failed to create agent");
    }
  }

  async function removeAgent(agentID: string) {
    await api.deleteAgent(agentID, token);
    mutate();
  }

  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Agents</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">
          AI workers assigned to projects. Each agent has a role, provider, and model.
        </p>
      </div>

      {/* Project selector */}
      <div className="mb-5">
        <label className="mb-2 block text-sm text-slate-300">Select project</label>
        <select
          value={selectedProject}
          onChange={(e) => setSelectedProject(e.target.value)}
          className="w-full max-w-md rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
        >
          <option value="">— Choose a project —</option>
          {projects.map((p: Project) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
      </div>

      {selectedProject && (
        <>
          {/* Create agent form */}
          <div className="mb-6 rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5">
            <div className="mb-4 flex items-center gap-2">
              <Bot size={18} className="text-[var(--accent)]" />
              <h3 className="font-mono font-semibold">Add Agent</h3>
            </div>
            <form className="grid gap-3 md:grid-cols-[1fr_1fr_1fr_1fr_auto]" onSubmit={createAgent}>
              <input
                value={agentName}
                onChange={(e) => setAgentName(e.target.value)}
                placeholder="Agent name"
                className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
              />
              <select
                value={agentRole}
                onChange={(e) => setAgentRole(e.target.value)}
                className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
              >
                {AGENT_ROLES.map((role) => (
                  <option key={role} value={role}>
                    {role}
                  </option>
                ))}
              </select>
              <select
                value={agentProvider}
                onChange={(e) => setAgentProvider(e.target.value)}
                className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
              >
                {PROVIDERS.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
              <input
                value={agentModel}
                onChange={(e) => setAgentModel(e.target.value)}
                placeholder="Model (e.g. gpt-4o)"
                className="rounded-md border border-[var(--border)] bg-slate-950 px-3 py-2 text-sm text-white"
              />
              <button
                className="flex items-center justify-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90"
                type="submit"
              >
                <Plus size={16} />
                Add
              </button>
            </form>
            {formError && (
              <p className="mt-3 rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">
                {formError}
              </p>
            )}
          </div>

          {/* Agents grid */}
          {agents.length === 0 ? (
            <EmptyState
              icon={Bot}
              title="No agents yet"
              description="Create an agent above to assign it to tasks in this project."
            />
          ) : (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {agents.map((agent: Agent) => (
                <article
                  key={agent.id}
                  className="group rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]/40"
                >
                  <div className="mb-3 flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <div className="grid size-10 place-items-center rounded-lg bg-[var(--accent)]/10 text-[var(--accent)]">
                        <Bot size={20} />
                      </div>
                      <div>
                        <h3 className="font-mono font-semibold">{agent.name}</h3>
                        <p className="text-xs text-[var(--muted)]">{agent.provider}/{agent.model}</p>
                      </div>
                    </div>
                    <button
                      onClick={() => removeAgent(agent.id)}
                      className="rounded-md p-1.5 text-slate-500 opacity-0 transition hover:bg-red-950/40 hover:text-red-300 group-hover:opacity-100"
                      title="Remove agent"
                      type="button"
                    >
                      <Trash2 size={15} />
                    </button>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Badge value={agent.role} />
                    <Badge value={agent.status || "idle"} />
                    {agent.level > 0 && (
                      <span className="rounded border border-[var(--border)] px-2 py-0.5 text-xs text-[var(--muted)]">
                        Level {agent.level}
                      </span>
                    )}
                  </div>
                </article>
              ))}
            </div>
          )}
        </>
      )}
    </DashboardLayout>
  );
}
