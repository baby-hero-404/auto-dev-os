"use client";

import { useMemo, useState } from "react";
import useSWR from "swr";
import { Bot, Loader2, Plus, Sparkles } from "lucide-react";
import { Toaster, toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Agent } from "@/lib/types";
import { EmptyState } from "@/components/ui/empty-state";
import { HireAgentWizard, type HireAgentPayload } from "@/components/dashboard/hire-agent-wizard";

import { DEFAULT_FLEET } from "./members/DEFAULT_FLEET";
import { Metric } from "./members/Metric";
import { AgentCardsSkeleton } from "./members/AgentCardsSkeleton";
import { AgentCard } from "./members/AgentCard";
import { providerSummary, seedFleetTitle } from "./members/utils";

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
              <AgentCard
                key={agent.id}
                agent={agent}
                assignedProjectNames={assignedProjectNames}
                isAutoJoin={isAutoJoin}
                assignableProjects={assignableProjects}
                confirmingAgentID={confirmingAgentID}
                assigningValue={assigningMap[agent.id] || ""}
                agentActionError={agentActionErrors[agent.id]}
                onSetConfirmingAgentID={setConfirmingAgentID}
                onRemoveAgent={removeAgent}
                onCancelConfirm={() => setConfirmingAgentID("")}
                onAssignValueChange={(val) => setAssigningMap((prev) => ({ ...prev, [agent.id]: val }))}
                onAssignSubmit={() => assignAgentToProject(agent)}
              />
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
