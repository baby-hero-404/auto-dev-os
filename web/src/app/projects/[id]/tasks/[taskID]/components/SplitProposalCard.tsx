"use client";

import { useState } from "react";
import { GitFork, Check, X, Loader2, FileCode, ShieldAlert, Database } from "lucide-react";
import { tasks as tasksApi } from "@/lib/api/projects";
import { useTaskDetail } from "./TaskDetailContext";
import type { ChildTaskSpec } from "@/lib/types";

export function SplitProposalCard() {
  const { task, taskID, token, mutateWorkflow, setError } = useTaskDetail();
  const [submitting, setSubmitting] = useState<"approve" | "reject" | null>(null);

  if (!task || task.status !== "planning_split") {
    return null;
  }

  const score = task.complexity_score;
  const initialChildren: ChildTaskSpec[] = (task.analysis?.child_specs as ChildTaskSpec[]) || [];
  const [children, setChildren] = useState<ChildTaskSpec[]>(initialChildren);

  const handleApprove = async () => {
    if (!token) return;
    setSubmitting("approve");
    try {
      await tasksApi.approveSplit(taskID, token, children.length > 0 ? children : undefined);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to approve split proposal");
    } finally {
      setSubmitting(null);
    }
  };

  const handleReject = async () => {
    if (!token) return;
    setSubmitting("reject");
    try {
      await tasksApi.rejectSplit(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to reject split proposal");
    } finally {
      setSubmitting(null);
    }
  };

  const updateChild = (index: number, key: keyof ChildTaskSpec, value: string) => {
    const updated = [...children];
    updated[index] = { ...updated[index], [key]: value };
    setChildren(updated);
  };

  return (
    <div className="bg-card border border-amber-500/30 dark:border-amber-500/20 rounded-2xl p-6 shadow-md mb-6 animate-fade-in">
      <div className="flex items-center justify-between pb-4 border-b border-stroke/10 mb-4">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-amber-500/10 text-amber-500">
            <GitFork className="w-5 h-5" />
          </div>
          <div>
            <h3 className="text-base font-bold text-foreground">Task Decomposition Proposal</h3>
            <p className="text-xs text-content-muted">
              This task exceeds the single-run complexity threshold. Review and approve the proposed sub-task decomposition.
            </p>
          </div>
        </div>
        <span className="px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
          Operator Review Needed
        </span>
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

      {children.length > 0 && (
        <div className="space-y-3 mb-5">
          <div className="text-xs font-bold text-foreground uppercase tracking-wider">
            Proposed Sub-Tasks ({children.length})
          </div>
          {children.map((child, idx) => (
            <div key={idx} className="p-4 rounded-xl bg-surface/20 border border-stroke/10 space-y-2">
              <div className="flex items-center gap-2">
                <span className="w-6 h-6 rounded-full bg-amber-500/20 text-amber-400 text-xs font-bold flex items-center justify-center shrink-0">
                  {idx + 1}
                </span>
                <input
                  type="text"
                  value={child.title}
                  onChange={(e) => updateChild(idx, "title", e.target.value)}
                  className="w-full bg-background border border-stroke/20 rounded-lg px-3 py-1.5 text-sm font-medium text-foreground focus:outline-none focus:border-amber-500"
                />
              </div>
              <textarea
                value={child.instructions}
                onChange={(e) => updateChild(idx, "instructions", e.target.value)}
                rows={2}
                className="w-full bg-background border border-stroke/20 rounded-lg p-2.5 text-xs text-content-muted focus:outline-none focus:border-amber-500 font-mono"
              />
            </div>
          ))}
        </div>
      )}

      <div className="flex items-center justify-end gap-3 pt-3 border-t border-stroke/10">
        <button
          onClick={handleReject}
          disabled={submitting !== null}
          className="px-4 py-2 rounded-xl border border-stroke/20 text-sm font-semibold text-content-muted hover:text-foreground hover:bg-surface/50 transition-all flex items-center gap-2 disabled:opacity-50"
        >
          {submitting === "reject" ? <Loader2 className="w-4 h-4 animate-spin" /> : <X className="w-4 h-4" />}
          Reject Split (Single Task)
        </button>

        <button
          onClick={handleApprove}
          disabled={submitting !== null}
          className="px-5 py-2 rounded-xl bg-amber-500 hover:bg-amber-600 text-slate-950 font-bold text-sm shadow-md hover:shadow-amber-500/20 transition-all flex items-center gap-2 disabled:opacity-50"
        >
          {submitting === "approve" ? <Loader2 className="w-4 h-4 animate-spin" /> : <Check className="w-4 h-4" />}
          Approve Decomposed Execution
        </button>
      </div>
    </div>
  );
}
