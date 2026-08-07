"use client";

import { useTaskDetail } from "../TaskDetailContext";
import { GitFork, FileCode, ShieldAlert, Database } from "lucide-react";
import type { ChildTaskSpec } from "@/lib/types";

/**
 * Read-only sub-view of CodingProgressView, shown whenever the task payload
 * has a non-empty proposed split. No approve/reject buttons — decomposition
 * is an autopilot decision, not a human approval gate (design.md's "Redundant
 * Component Consolidation" section). Contrast with the legacy SplitProposalCard,
 * which still exposes approve/reject until it's deleted in Phase 5.
 */
export function SplitProposalView() {
  const { task } = useTaskDetail();
  const children: ChildTaskSpec[] = (task?.analysis?.child_specs as ChildTaskSpec[]) || [];

  if (children.length === 0) return null;

  const score = task?.complexity_score;

  return (
    <div className="bg-card border border-amber-500/30 dark:border-amber-500/20 rounded-2xl p-6 shadow-md mb-6 animate-fade-in">
      <div className="flex items-center justify-between pb-4 border-b border-stroke/10 mb-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-500">
            <GitFork className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-base font-bold text-foreground">Task Decomposition</h3>
            <p className="text-xs text-content-muted">
              This task exceeded the single-run complexity threshold and was split into sub-tasks automatically.
            </p>
          </div>
        </div>
      </div>

      {score && (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-5">
          <div className="bg-surface/30 p-3 rounded-xl border border-stroke/10">
            <div className="text-xs text-content-muted mb-1">Complexity Score</div>
            <div className="text-lg font-bold text-amber-400">{score.total} points</div>
          </div>
          <div className="bg-surface/30 p-3 rounded-xl border border-stroke/10">
            <div className="text-xs text-content-muted mb-1 flex items-center gap-1.5">
              <FileCode className="w-3.5 h-3.5" /> Affected Files
            </div>
            <div className="text-lg font-bold text-foreground">{score.file_count} files</div>
          </div>
          <div className="bg-surface/30 p-3 rounded-xl border border-stroke/10">
            <div className="text-xs text-content-muted mb-1 flex items-center gap-1.5">
              <Database className="w-3.5 h-3.5" /> Data Migration
            </div>
            <div className={`text-sm font-semibold ${score.data_migration ? "text-amber-400" : "text-content-muted"}`}>
              {score.data_migration ? "Yes (High Risk)" : "No"}
            </div>
          </div>
          <div className="bg-surface/30 p-3 rounded-xl border border-stroke/10">
            <div className="text-xs text-content-muted mb-1 flex items-center gap-1.5">
              <ShieldAlert className="w-3.5 h-3.5" /> Security Sensitive
            </div>
            <div className={`text-sm font-semibold ${score.security_sensitive ? "text-red-400" : "text-content-muted"}`}>
              {score.security_sensitive ? "Yes" : "No"}
            </div>
          </div>
        </div>
      )}

      <div className="space-y-3">
        <div className="text-xs font-bold text-foreground uppercase tracking-wider">
          Sub-Tasks ({children.length})
        </div>
        {children.map((child, idx) => (
          <div key={idx} className="p-4 rounded-xl bg-surface/20 border border-stroke/10 space-y-2">
            <div className="flex items-center gap-2">
              <span className="w-6 h-6 rounded-full bg-amber-500/20 text-amber-400 text-xs font-bold flex items-center justify-center shrink-0">
                {idx + 1}
              </span>
              <span className="text-sm font-medium text-foreground">{child.title}</span>
            </div>
            <p className="text-xs text-content-muted font-mono pl-8">{child.instructions}</p>
          </div>
        ))}
      </div>
    </div>
  );
}
