import { useState } from "react";
import { Bot } from "lucide-react";
import type { Agent, Skill } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { ApiError } from "@/lib/api";

interface AgentSkillGridProps {
  projectAgents: Agent[];
  agentSkills: Record<string, Skill[]>;
  globalSkills: Skill[];
  onAssignSkill: (agentId: string, skillId: string) => Promise<void>;
}

export function AgentSkillGrid({
  projectAgents,
  agentSkills,
  globalSkills,
  onAssignSkill,
}: AgentSkillGridProps) {
  const [skillMap, setSkillMap] = useState<Record<string, string>>({});
  const [skillErrors, setSkillErrors] = useState<Record<string, string>>({});

  async function handleAssignSkill(agentID: string) {
    const skillID = skillMap[agentID];
    if (!skillID) return;
    setSkillErrors((p) => ({ ...p, [agentID]: "" }));
    try {
      await onAssignSkill(agentID, skillID);
      setSkillMap((p) => {
        const n = { ...p };
        delete n[agentID];
        return n;
      });
    } catch (err) {
      setSkillErrors((p) => ({ ...p, [agentID]: err instanceof ApiError ? err.message : "Failed to assign skill" }));
    }
  }

  return (
    <div className="rounded-lg border border-stroke bg-white dark:bg-slate-950 p-5">
      <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-2">
        <Bot size={18} className="text-brand-primary" />
        <h3 className="font-mono font-semibold text-foreground dark:text-white">Agent Skills Configuration</h3>
      </div>
      <div className="space-y-4">
        {projectAgents.length === 0 ? (
          <p className="text-xs text-content-muted italic">No agents assigned to this project yet.</p>
        ) : (
          projectAgents.map((agent) => {
            const assigned = agentSkills[agent.id] || [];
            const assignable = globalSkills.filter((gs) => !assigned.some((as) => as.id === gs.id));
            return (
              <div key={agent.id} className="rounded border border-stroke/40 bg-slate-50 dark:bg-slate-900 p-4 space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <h4 className="font-mono text-sm font-bold text-foreground dark:text-white">{agent.name}</h4>
                    <p className="text-xs text-content-muted uppercase font-mono tracking-wide">{agent.role}</p>
                  </div>
                  <Badge value={agent.autonomy_level} />
                </div>
                <div>
                  <span className="block text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted mb-1">Active Skills:</span>
                  {assigned.length === 0 ? (
                    <p className="text-xs text-content-muted italic">No skills assigned.</p>
                  ) : (
                    <div className="flex flex-wrap gap-1.5">
                      {assigned.map((s) => (
                        <span key={s.id} className="rounded bg-slate-100 dark:bg-slate-950 border border-stroke px-2 py-0.5 text-xs text-foreground dark:text-white" title={s.description}>
                          {s.name}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
                {assignable.length > 0 && (
                  <div className="space-y-2 border-t border-stroke/30 pt-2">
                    <div className="flex gap-2">
                      <select
                        value={skillMap[agent.id] || ""}
                        onChange={(e) => setSkillMap((p) => ({ ...p, [agent.id]: e.target.value }))}
                        className="flex-1 rounded border border-stroke bg-slate-50 dark:bg-slate-950 px-2 py-1 text-xs text-foreground dark:text-white focus:outline-none focus:border-brand-primary cursor-pointer"
                      >
                        <option value="">— Assign Skill —</option>
                        {assignable.map((s) => (
                          <option key={s.id} value={s.id}>
                            {s.name}
                          </option>
                        ))}
                      </select>
                      <button
                        onClick={() => handleAssignSkill(agent.id)}
                        disabled={!skillMap[agent.id]}
                        className="rounded bg-brand-primary px-3 py-1 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                        type="button"
                      >
                        Assign
                      </button>
                    </div>
                    {skillErrors[agent.id] && (
                      <p className="rounded border border-red-500/20 bg-red-950/40 p-2 text-xs text-red-200">
                        {skillErrors[agent.id]}
                      </p>
                    )}
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
