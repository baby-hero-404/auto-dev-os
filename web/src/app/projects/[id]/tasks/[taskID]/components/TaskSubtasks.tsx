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
    <div className="bg-card border border-stroke rounded-xl p-5">
      <div className="flex items-center justify-between mb-2.5">
        <span className="text-sm font-semibold">Subtasks</span>
        <span className="text-[13px] text-content-muted">{completedTasks} of {totalTasks} completed</span>
      </div>
      <div className="h-1.5 rounded-full bg-surface overflow-hidden mb-3.5">
        <div className="h-full rounded-full bg-brand-primary transition-all duration-300" style={{ width: `${progressPct}%` }}></div>
      </div>
      <div className="flex flex-col gap-1.5">
        {implementationItems.map((item, idx) => {
          const isDone = item.status === 'done';
          const isRunning = item.status === 'running';
          return (
            <div key={item.id || idx} className="flex items-center gap-2.5 px-3 py-2.5 rounded-lg border" style={{
              background: isDone ? 'var(--brand-primary-dim, rgba(var(--brand-primary-rgb), 0.08))' : isRunning ? 'var(--brand-primary-dim, rgba(var(--brand-primary-rgb), 0.08))' : 'var(--card)',
              borderColor: (isDone || isRunning) ? '#90c5ff' : 'var(--stroke)',
            }}>
              <span className="inline-flex items-center justify-center w-4 h-4 rounded-full text-[10px] border-[1.5px]" style={{
                background: isDone ? 'var(--brand-primary)' : 'transparent',
                borderColor: isDone ? 'var(--brand-primary)' : '#cad5e2',
                color: 'var(--background)'
              }}>{isDone ? '✓' : ''}</span>
              <span className="text-sm flex-1" style={{ textDecoration: isDone ? 'line-through' : 'none' }}>
                {item.name}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
