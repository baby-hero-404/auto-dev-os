import { Loader2, Network, TrendingDown } from "lucide-react";
import type { KnowledgeEdge } from "@/lib/types";

interface MemoryInspectorProps {
  inspectingMemoryID: string;
  detailData: any;
  onClose: () => void;
}

export function MemoryInspector({
  inspectingMemoryID,
  detailData,
  onClose,
}: MemoryInspectorProps) {
  return (
    <div className="rounded-lg border border-stroke bg-slate-950 p-5 mt-4">
      <div className="mb-4 flex items-center justify-between border-b border-stroke pb-3">
        <h3 className="font-mono font-semibold flex items-center gap-2 text-white">
          <Network size={16} className="text-brand-primary" />
          Memory Inspector & Relations Graph
        </h3>
        <button
          onClick={onClose}
          className="text-xs text-content-muted hover:text-white cursor-pointer"
        >
          ✕ Close
        </button>
      </div>

      {!detailData ? (
        <div className="flex justify-center py-8">
          <Loader2 size={24} className="animate-spin text-brand-primary" />
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-[1fr_250px]">
          <div className="flex flex-col gap-3">
            <div>
              <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted">Summary</span>
              <p className="text-sm text-slate-100 font-medium">{detailData.memory.summary}</p>
            </div>
            <div>
              <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted">Content Details</span>
              <pre className="mt-1 rounded bg-slate-900 border border-stroke p-3 font-mono text-xs text-slate-300 overflow-x-auto whitespace-pre-wrap">
                {detailData.memory.content}
              </pre>
            </div>
            {detailData.memory.tags && detailData.memory.tags.length > 0 && (
              <div>
                <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted block mb-1">Tags</span>
                <div className="flex flex-wrap gap-1">
                  {detailData.memory.tags.map((t: string) => (
                    <span key={t} className="rounded bg-slate-800 border border-stroke px-1.5 py-0.5 text-[10px] text-slate-300 font-mono">
                      #{t}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>

          <div className="border-t border-stroke pt-4 md:border-t-0 md:pt-0 md:border-l md:pl-4 flex flex-col gap-4">
            <div>
              <h4 className="text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted mb-2">
                Connected Entities
              </h4>
              {!detailData.edges || detailData.edges.length === 0 ? (
                <p className="text-xs text-content-muted italic">No knowledge connections detected.</p>
              ) : (
                <div className="space-y-2">
                  {detailData.edges.map((edge: KnowledgeEdge) => (
                    <div key={edge.id} className="rounded border border-stroke bg-slate-900/60 p-2 text-xs">
                      <div className="font-mono font-semibold text-brand-primary">{edge.relation}</div>
                      <div className="text-[10px] text-content-muted mt-0.5">
                        Weight: {edge.weight.toFixed(2)}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="space-y-2 text-xs border-t border-stroke pt-3">
              <div className="flex justify-between">
                <span className="text-content-muted">Importance:</span>
                <span className="font-mono text-slate-300 font-semibold">{detailData.memory.access_count}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-content-muted">Decay math:</span>
                <span className="font-mono text-slate-300 font-semibold flex items-center gap-1">
                  <TrendingDown size={12} className="text-amber-400" />
                  {detailData.memory.decay_score.toFixed(3)}
                </span>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
