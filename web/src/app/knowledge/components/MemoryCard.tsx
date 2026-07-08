import { Calendar, Trash2 } from "lucide-react";
import type { EpisodicMemory } from "@/lib/types";

export function MemoryCard({
  memory,
  score,
  isAdmin,
  onDelete,
  onInspect,
  isInspecting,
}: {
  memory: EpisodicMemory;
  score?: number;
  isAdmin: boolean;
  onDelete: (id: string) => void;
  onInspect: (id: string) => void;
  isInspecting: boolean;
}) {
  const isError = memory.category === "error";
  const isSuccess = memory.category === "success";

  const tierColors = {
    working: "border-slate-800 text-slate-300 bg-slate-900/40",
    episodic: "border-cyan-500/20 text-cyan-300 bg-cyan-950/10",
    semantic: "border-amber-500/20 text-amber-300 bg-amber-950/10",
    procedural: "border-emerald-500/20 text-emerald-300 bg-emerald-950/10",
  };

  return (
    <article
      className={`group rounded-lg border p-4 transition flex flex-col justify-between cursor-pointer ${
        isInspecting
          ? "border-brand-primary bg-slate-950/80 shadow-[0_0_12px_rgba(235,166,90,0.15)]"
          : "border-stroke bg-panel hover:border-brand-primary/40"
      }`}
      onClick={() => onInspect(memory.id)}
    >
      <div>
        <div className="mb-2 flex items-start justify-between">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className={`rounded px-1.5 py-0.5 text-[9px] font-mono font-bold uppercase tracking-wider border ${tierColors[memory.tier as keyof typeof tierColors] || "border-slate-800"}`}>
              {memory.tier}
            </span>
            <span className={`rounded-full px-2 py-0.5 text-[9px] font-mono ${
              isError
                ? "bg-red-400/10 text-red-300 border border-red-500/20"
                : isSuccess
                ? "bg-emerald-400/10 text-emerald-300 border border-emerald-500/20"
                : "bg-slate-800 text-slate-300 border border-slate-700/50"
            }`}>
              {memory.category}
            </span>
            {score !== undefined && (
              <span className="rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/20 px-1.5 py-0.5 text-[9px] font-mono">
                RRF Rnk: {score.toFixed(4)}
              </span>
            )}
          </div>
          {isAdmin && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDelete(memory.id);
              }}
              className="rounded-md p-1 text-slate-500 opacity-0 transition hover:bg-red-950/40 hover:text-red-300 group-hover:opacity-100 cursor-pointer"
              title="Prune memory"
            >
              <Trash2 size={13} />
            </button>
          )}
        </div>

        <h4 className="font-mono text-xs font-semibold text-slate-100 line-clamp-2 mt-1">{memory.summary}</h4>
        <p className="mt-2 text-xs text-content-muted line-clamp-3 leading-relaxed">{memory.content}</p>
      </div>

      <div className="mt-3 border-t border-stroke/60 pt-2 flex items-center justify-between text-[10px] text-content-muted">
        <span className="flex items-center gap-1">
          <Calendar size={11} />
          {new Date(memory.created_at).toLocaleDateString()}
        </span>
        <span className="font-mono">Seen: {memory.access_count}</span>
      </div>
    </article>
  );
}
