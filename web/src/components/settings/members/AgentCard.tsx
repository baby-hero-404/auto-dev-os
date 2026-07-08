import { ArrowRight, Bot, ChevronDown, Trash2 } from "lucide-react";
import type { Agent, Project } from "@/lib/types";
import { Badge } from "@/components/ui/badge";

interface AgentCardProps {
  agent: Agent;
  assignedProjectNames: string[];
  isAutoJoin: boolean;
  assignableProjects: Project[];
  confirmingAgentID: string;
  assigningValue: string;
  agentActionError?: string;
  onSetConfirmingAgentID: (id: string) => void;
  onRemoveAgent: (id: string) => void;
  onCancelConfirm: () => void;
  onAssignValueChange: (val: string) => void;
  onAssignSubmit: () => void;
}

export function AgentCard({
  agent,
  assignedProjectNames,
  isAutoJoin,
  assignableProjects,
  confirmingAgentID,
  assigningValue,
  agentActionError,
  onSetConfirmingAgentID,
  onRemoveAgent,
  onCancelConfirm,
  onAssignValueChange,
  onAssignSubmit,
}: AgentCardProps) {
  return (
    <article className="glass-panel glow-on-hover group flex flex-col justify-between rounded-lg p-5">
      <div>
        <div className="mb-3 flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-center gap-3">
            <div className="grid size-10 shrink-0 place-items-center rounded-lg bg-brand-primary/10 text-brand-primary">
              <Bot size={20} />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h3 className="truncate font-semibold text-foreground">{agent.name}</h3>
                <span className={`inline-flex items-center gap-1 shrink-0 rounded-full px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wide border ${
                  agent.status === "offline"
                    ? "bg-slate-500/10 text-slate-400 border-slate-500/20"
                    : ["busy", "assigned", "running"].includes(agent.status)
                    ? "bg-amber-500/10 text-amber-500 border-amber-500/20"
                    : "bg-emerald-500/10 text-emerald-500 border-emerald-500/20"
                }`}>
                  <span className={`h-1.5 w-1.5 rounded-full ${
                    agent.status === "offline" ? "bg-slate-400" : ["busy", "assigned", "running"].includes(agent.status) ? "bg-amber-500 animate-pulse" : "bg-emerald-500"
                  }`} />
                  {agent.status === "offline" ? "Offline" : ["busy", "assigned", "running"].includes(agent.status) ? "Working" : "Free"}
                </span>
              </div>
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
            onClick={() => onSetConfirmingAgentID(agent.id)}
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

        {agentActionError && (
          <p className="mt-3 rounded border border-red-500/20 bg-red-500/10 p-2 text-xs text-red-600 dark:text-red-400 font-medium">{agentActionError}</p>
        )}
      </div>

      {confirmingAgentID === agent.id && (
        <div className="mt-4 rounded-md border border-red-500/20 bg-red-500/10 p-3">
          <p className="text-xs text-red-600 dark:text-red-300 font-medium">Dismiss this organization agent?</p>
          <div className="mt-3 flex gap-2">
            <button onClick={() => onRemoveAgent(agent.id)} className="rounded bg-danger px-3 py-1 text-xs font-semibold text-white hover:opacity-90 cursor-pointer" type="button">
              Dismiss
            </button>
            <button onClick={onCancelConfirm} className="rounded border border-stroke px-3 py-1 text-xs font-semibold text-foreground hover:bg-surface cursor-pointer" type="button">
              Cancel
            </button>
          </div>
        </div>
      )}

      {!isAutoJoin && assignableProjects.length > 0 && (
        <div className="mt-4 flex gap-2 border-t border-stroke pt-3">
          <div className="relative flex-1">
            <select
              value={assigningValue}
              onChange={(e) => onAssignValueChange(e.target.value)}
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
            onClick={onAssignSubmit}
            disabled={!assigningValue}
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
}
