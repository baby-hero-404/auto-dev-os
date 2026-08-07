"use client";

import { useState } from "react";
import { FileText, ChevronDown, ChevronRight } from "lucide-react";
import type { TaskEvent, FileChangedPayload } from "@/lib/types/task-event";

interface FileStat {
  path: string;
  additions: number;
  deletions: number;
  changeCount: number;
}

// Groups every file.changed event by path (IDE Source Control panel style):
// consecutive/repeated edits to the same file collapse into one row with
// summed +/- counts, rather than one timeline card per edit.
function groupByFile(events: TaskEvent[]): FileStat[] {
  const byPath = new Map<string, FileStat>();
  for (const event of events) {
    if (event.type !== "file.changed") continue;
    const p = event.payload as unknown as FileChangedPayload;
    if (!p.path) continue;
    const existing = byPath.get(p.path);
    if (existing) {
      existing.additions += p.additions;
      existing.deletions += p.deletions;
      existing.changeCount += 1;
    } else {
      byPath.set(p.path, { path: p.path, additions: p.additions, deletions: p.deletions, changeCount: 1 });
    }
  }
  return Array.from(byPath.values()).sort((a, b) => b.additions + b.deletions - (a.additions + a.deletions));
}

// A compact green/red proportional bar giving an at-a-glance diff shape for
// a file, the way GitHub's PR file list does, without needing real diff
// content (task_events only carries +/- counts, not line-level diffs).
function DiffBar({ additions, deletions }: { additions: number; deletions: number }) {
  const total = additions + deletions;
  if (total === 0) return null;
  const addPct = (additions / total) * 100;
  return (
    <span className="flex h-2 w-16 shrink-0 overflow-hidden rounded-sm bg-stroke/40" title={`+${additions} / -${deletions}`}>
      <span className="h-full bg-emerald-500/80" style={{ width: `${addPct}%` }} />
      <span className="h-full bg-red-500/70" style={{ width: `${100 - addPct}%` }} />
    </span>
  );
}

export function FilesChangedPanel({ events }: { events: TaskEvent[] }) {
  const [collapsed, setCollapsed] = useState(false);
  const files = groupByFile(events);
  if (files.length === 0) return null;

  const totalAdd = files.reduce((sum, f) => sum + f.additions, 0);
  const totalDel = files.reduce((sum, f) => sum + f.deletions, 0);

  return (
    <div className="mb-2 rounded-md border border-stroke bg-card/50 text-xs">
      <button
        type="button"
        onClick={() => setCollapsed((v) => !v)}
        className="flex w-full items-center justify-between gap-2 p-2 text-left"
      >
        <span className="flex items-center gap-1.5 text-content">
          {collapsed ? <ChevronRight className="h-3.5 w-3.5" /> : <ChevronDown className="h-3.5 w-3.5" />}
          <FileText className="h-3.5 w-3.5" />
          {files.length} file{files.length === 1 ? "" : "s"} changed
        </span>
        <span className="text-content-secondary">
          <span className="text-emerald-500">+{totalAdd}</span>{" "}
          <span className="text-red-500">-{totalDel}</span>
        </span>
      </button>
      {!collapsed && (
        <div className="divide-y divide-stroke/50 border-t border-stroke">
          {files.map((f) => (
            <div key={f.path} className="flex items-center justify-between gap-2 px-2 py-1.5">
              <span className="truncate font-mono text-[11px] text-content" title={f.path}>
                {f.path}
                {f.changeCount > 1 && (
                  <span className="ml-1 text-content-secondary">({f.changeCount}x)</span>
                )}
              </span>
              <span className="flex shrink-0 items-center gap-2">
                <span className="text-[10px] text-content-secondary">
                  <span className="text-emerald-500">+{f.additions}</span>{" "}
                  <span className="text-red-500">-{f.deletions}</span>
                </span>
                <DiffBar additions={f.additions} deletions={f.deletions} />
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
