import { compactNumber, formatCost } from "../utils";

export function AgentPerformanceTable({ agentStats }: { agentStats: any[] }) {
  return (
    <section className="overflow-hidden rounded-lg border border-stroke bg-panel">
      <div className="border-b border-stroke p-5">
        <h3 className="font-mono font-semibold">Agent Performance</h3>
        <p className="text-sm text-content-muted">Per-agent metrics, success rates, and token consumption.</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-sm">
          <thead className="border-b border-stroke text-xs uppercase tracking-wide text-content-muted">
            <tr>
              <th className="px-4 py-3">Agent</th>
              <th className="px-4 py-3">Role</th>
              <th className="px-4 py-3">Status</th>
              <th className="px-4 py-3">Tasks</th>
              <th className="px-4 py-3">Success</th>
              <th className="px-4 py-3">Retries</th>
              <th className="px-4 py-3">Tokens</th>
              <th className="px-4 py-3">Cost</th>
            </tr>
          </thead>
          <tbody>
            {agentStats?.map((agent) => (
              <tr key={agent.agent_id} className="border-b border-stroke/60 transition hover:bg-slate-900/50">
                <td className="px-4 py-3">
                  <div className="font-medium">{agent.agent_name}</div>
                  <div className="mt-1">
                    <span className={`inline-flex items-center gap-0.5 rounded px-1.5 py-0.2 text-[9px] font-bold uppercase ${
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
                </td>
                <td className="px-4 py-3 capitalize">{agent.role}</td>
                <td className="px-4 py-3">
                  <span className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium ${
                    agent.status === "idle" ? "bg-slate-700/20 text-slate-400 border-slate-700/30" :
                    agent.status === "assigned" ? "bg-blue-500/10 text-blue-300 border-blue-500/20" :
                    agent.status === "running" || agent.status === "busy" ? "bg-emerald-400/10 text-emerald-300 border-emerald-400/20" :
                    agent.status === "offline" ? "bg-red-500/10 text-red-400 border-red-500/20" :
                    "bg-slate-700/20 text-slate-400 border-slate-700/30"
                  }`}>
                    <span className={`size-1.5 rounded-full ${
                      agent.status === "idle" ? "bg-slate-500" :
                      agent.status === "assigned" ? "bg-blue-400 animate-pulse" :
                      agent.status === "running" || agent.status === "busy" ? "bg-emerald-400 animate-pulse" :
                      agent.status === "offline" ? "bg-red-500" :
                      "bg-slate-500"
                    }`} />
                    {agent.status || "idle"}
                  </span>
                </td>
                <td className="px-4 py-3 font-mono">{agent.task_count}</td>
                <td className="px-4 py-3">
                  <div className="flex items-center gap-2">
                    <div className="h-1.5 w-16 rounded-full bg-slate-700">
                      <div
                        className="h-1.5 rounded-full bg-brand-primary transition-all"
                        style={{ width: `${Math.min(agent.success_rate, 100)}%` }}
                      />
                    </div>
                    <span className="text-xs font-mono">{Math.round(agent.success_rate)}%</span>
                  </div>
                </td>
                <td className="px-4 py-3 font-mono">{agent.retry_count}</td>
                <td className="px-4 py-3 font-mono">{compactNumber(agent.total_tokens)}</td>
                <td className="px-4 py-3 font-mono">{formatCost(agent.total_cost_usd)}</td>
              </tr>
            ))}
            {agentStats?.length === 0 && (
              <tr>
                <td className="px-4 py-8 text-center text-content-muted" colSpan={8}>
                  No agent data available yet.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
