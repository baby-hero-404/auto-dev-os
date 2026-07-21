"use client";

import { useTaskDetail } from "./TaskDetailContext";

export function TaskSubtasks() {
  const { task, implementationItems } = useTaskDetail();
  const st = task?.status || "todo";
  
  if (st === 'todo' || st === 'merged' || st === 'failed') {
    return null;
  }

  if (!implementationItems || implementationItems.length === 0) {
    if (st === 'coding' || st === 'testing' || st === 'reviewing' || st === 'fixing') {
      return (
        <div className="bg-card border border-stroke rounded-xl p-5">
          <div className="flex items-center justify-between mb-2.5">
            <span className="text-sm font-semibold">Subtasks</span>
          </div>
          <div className="text-xs text-content-muted italic">Waiting for implementation items to be generated...</div>
        </div>
      );
    }
    return null;
  }

  const completedTasks = implementationItems.filter(i => i.status === 'done').length;
  const totalTasks = implementationItems.length;
  const progressPct = totalTasks > 0 ? (completedTasks / totalTasks * 100) : 0;

  return (
    <div className="bg-card border border-stroke/10 rounded-2xl p-5.5 shadow-sm hover:shadow-md transition-all duration-200">
      <div className="flex items-center justify-between mb-3">
        <span className="text-sm font-bold text-foreground tracking-wide">Subtasks</span>
        <span className="text-xs font-semibold text-content-muted">{completedTasks} of {totalTasks} completed</span>
      </div>
      <div className="h-2 rounded-full bg-slate-500/10 overflow-hidden mb-4 shadow-inner">
        <div className="h-full rounded-full bg-gradient-to-r from-blue-500 via-indigo-500 to-emerald-500 transition-all duration-500" style={{ width: `${progressPct}%` }}></div>
      </div>
      <div className="flex flex-col gap-2">
        {implementationItems.map((item, idx) => {
          const isDone = item.status === 'done';
          const isRunning = item.status === 'running';
          
          let itemClass = "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-stroke/10 bg-slate-500/[0.02] hover:bg-slate-500/5 text-slate-700 dark:text-slate-200 transition-all duration-150";
          let indicator = <span className="w-5 h-5 rounded-full border border-stroke/30 bg-background shrink-0 transition-colors"></span>;
          
          if (isDone) {
            itemClass = "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-emerald-500/20 bg-emerald-500/[0.04] text-emerald-800 dark:text-emerald-300 transition-all duration-150";
            indicator = (
              <span className="inline-flex items-center justify-center w-5 h-5 rounded-full text-xs font-bold bg-emerald-500 text-white shadow-sm shrink-0 animate-scale-in">
                ✓
              </span>
            );
          } else if (isRunning) {
            itemClass = "flex items-center gap-3 px-3.5 py-3 rounded-xl border border-blue-500/25 bg-blue-500/5 text-blue-800 dark:text-blue-300 transition-all duration-150 shadow-sm shadow-blue-500/5";
            indicator = (
              <span className="w-5 h-5 rounded-full border-2 border-blue-500/20 border-t-blue-500 animate-spin shrink-0"></span>
            );
          }
          
          return (
            <div key={item.id || idx} className={itemClass}>
              {indicator}
              <span className={`text-xs md:text-sm flex-1 leading-normal font-medium ${isDone ? 'line-through opacity-60' : ''}`}>
                {item.name}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
