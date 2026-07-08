import { Bot, Check } from "lucide-react";
import type { Agent } from "@/lib/types";

interface AgentSelectionProps {
  agents: Agent[];
  agentID: string;
  setAgentID: (id: string) => void;
  isSubmitting: boolean;
}

export function AgentSelection({ agents, agentID, setAgentID, isSubmitting }: AgentSelectionProps) {
  return (
    <div className="space-y-2 max-h-[175px] overflow-y-auto pr-1.5 scrollbar-thin flex flex-col gap-1.5" role="radiogroup" aria-label="Assign Agent">
      <button
        type="button"
        onClick={() => setAgentID("")}
        role="radio"
        aria-checked={agentID === ""}
        className={`flex w-full items-center gap-3.5 rounded-lg border p-3 text-left text-xs transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus ${
          agentID === ""
            ? "border-brand-primary/45 bg-brand-primary-muted text-foreground font-semibold shadow-sm"
            : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
        }`}
        disabled={isSubmitting}
      >
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-card text-brand-primary border border-stroke shadow-xs">
          <Bot size={16} aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="font-semibold text-foreground flex items-center justify-between">
            <span>Auto-assign</span>
            {agentID === "" && <Check size={14} className="text-brand-primary" />}
          </div>
          <div className="text-[10px] text-content-muted mt-0.5">Let the system select the best agent</div>
        </div>
      </button>

      {agents.map((agent) => {
        const isSelected = agentID === agent.id;
        const initials = agent.name.split(/\s+/).map((n) => n[0]).join("").slice(0, 2).toUpperCase();
        const isBusy = ["busy", "assigned", "running"].includes(agent.status);
        const isOffline = agent.status === "offline";
        return (
          <button
            key={agent.id}
            type="button"
            onClick={() => setAgentID(agent.id)}
            role="radio"
            aria-checked={isSelected}
            className={`flex w-full items-center gap-3.5 rounded-lg border p-3 text-left text-xs transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus ${
              isSelected
                ? "border-brand-primary/45 bg-brand-primary-muted text-foreground font-semibold shadow-sm"
                : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
            }`}
            disabled={isSubmitting}
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-card font-mono font-bold text-foreground border border-stroke shadow-xs text-xs">
              {initials}
            </div>
            <div className="min-w-0 flex-1">
              <div className="font-semibold text-foreground flex items-center justify-between">
                <div className="flex items-center gap-2 min-w-0">
                  <span className="truncate">{agent.name}</span>
                  <span
                    className={`inline-flex items-center gap-1 shrink-0 rounded-full px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wide border ${
                      isOffline
                        ? "bg-slate-500/10 text-slate-400 border-slate-500/20"
                        : isBusy
                        ? "bg-amber-500/10 text-amber-500 border-amber-500/20"
                        : "bg-emerald-500/10 text-emerald-500 border-emerald-500/20"
                    }`}
                  >
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${
                        isOffline ? "bg-slate-400" : isBusy ? "bg-amber-500 animate-pulse" : "bg-emerald-500"
                      }`}
                    />
                    {isOffline ? "Offline" : isBusy ? "Working" : "Free"}
                  </span>
                </div>
                {isSelected && <Check size={14} className="text-brand-primary shrink-0" />}
              </div>
              <div className="truncate text-[10px] text-content-muted mt-0.5">
                Role: {agent.role} • {agent.model_level_group || "Default Model Level Group"}
              </div>
            </div>
          </button>
        );
      })}
    </div>
  );
}
