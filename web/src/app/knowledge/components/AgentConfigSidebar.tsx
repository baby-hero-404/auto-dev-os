import { Loader2 } from "lucide-react";

interface AgentConfigSidebarProps {
  loadingAgents: boolean;
  orgAgents: any[];
  activeAgentID: string;
  selectedAgent: any;
  onSelectAgent: (id: string) => void;
}

export function AgentConfigSidebar({
  loadingAgents,
  orgAgents,
  activeAgentID,
  selectedAgent,
  onSelectAgent,
}: AgentConfigSidebarProps) {
  return (
    <aside className="rounded-lg border border-stroke bg-panel p-4 flex flex-col gap-4">
      <div>
        <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-2">
          Select Agent
        </h3>
        {loadingAgents ? (
          <div className="flex items-center gap-2 text-sm text-content-muted py-2">
            <Loader2 size={16} className="animate-spin" />
            Loading agents...
          </div>
        ) : orgAgents.length === 0 ? (
          <p className="text-xs text-content-muted italic">No agents available.</p>
        ) : (
          <div className="space-y-1">
            {orgAgents.map((agent) => (
              <button
                key={agent.id}
                onClick={() => onSelectAgent(agent.id)}
                className={`w-full text-left rounded-md px-3 py-2 text-xs font-mono flex items-center justify-between transition cursor-pointer ${
                  activeAgentID === agent.id
                    ? "bg-brand-primary text-slate-950 font-bold"
                    : "text-slate-300 hover:bg-slate-800"
                }`}
              >
                <span>{agent.name}</span>
                <span className="opacity-70 text-[9px] uppercase">{agent.role}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      {selectedAgent && (
        <div className="border-t border-stroke pt-4">
          <h4 className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-2">
            Agent Config
          </h4>
          <div className="space-y-1.5 text-xs">
            <div className="flex justify-between items-center">
              <span className="text-content-muted">Route:</span>
              <span className={`inline-flex items-center gap-0.5 rounded px-1.5 py-0.2 text-[10px] font-bold uppercase ${
                selectedAgent.model_level_group === "fast"
                  ? "bg-amber-500/10 text-amber-500 border border-amber-500/20"
                  : selectedAgent.model_level_group === "powerful"
                  ? "bg-purple-500/10 text-purple-500 border border-purple-500/20"
                  : "bg-blue-500/10 text-blue-500 border border-blue-500/20"
              }`}>
                {selectedAgent.model_level_group === "fast" && "⚡ "}
                {selectedAgent.model_level_group === "balanced" && "⚖️ "}
                {selectedAgent.model_level_group === "powerful" && "🚀 "}
                {selectedAgent.model_level_group}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-content-muted">Autonomy:</span>
              <span className="font-mono text-slate-200 capitalize">
                {selectedAgent.autonomy_level?.replace("_", " ")}
              </span>
            </div>
          </div>
        </div>
      )}
    </aside>
  );
}
