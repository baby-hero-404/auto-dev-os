"use client";

import { useRouter } from "next/navigation";
import { useTaskDetail } from "./TaskDetailContext";

export function TaskSidebar() {
  const router = useRouter();
  const { projectID, task, workflow, cancel, deleteTask, workflowSteps, latest, implementationItems, stepDurations } = useTaskDetail();
  const st = task?.status || "todo";
  
  const jobStatus = workflow?.job?.status?.toLowerCase();
  const canCancel = jobStatus === "running" || jobStatus === "paused" || jobStatus === "queued";

  const P: Record<string, [string, string, string, string, string]> = {
    todo:            ['Todo','todo','var(--surface)','var(--content-muted)','Preparation'],
    context_loading: ['Loading Context','context_loading','#e0efff','#005bb8','Preparation'],
    analyzing:       ['Analyzing','analyzing','#e0efff','#005bb8','Preparation'],
    planning:        ['Planning','planning','#e0efff','#005bb8','Preparation'],
    spec_review:     ['Spec Review','spec_review','#fef3c6','#795800','Preparation · Gate'],
    coding:          ['Coding','coding','#e0efff','#005bb8','Execution'],
    testing:         ['Testing','testing','#e0efff','#005bb8','Execution'],
    reviewing:       ['Reviewing','reviewing','#f3e8ff','#7f22fe','Execution'],
    fixing:          ['Fixing','fixing','#fff1e0','#b75000','Execution'],
    pr_ready:        ['PR Ready','pr_ready','#d9f5e7','#007956','Finalization'],
    human_review:    ['Human Review','human_review','#fef3c6','#795800','Finalization · Gate'],
    merged:          ['Merged','merged','#e6f4ea','#00590e','Finalization'],
    failed:          ['Failed','failed','#ffe2e2','#bf000f','Finalization'],
  };
  const [, code, , fg] = P[st] || P.todo;

  const phases = workflowSteps.map((step) => {
    const status = latest.get(step) || 'pending';
    const done = status === 'success' || status === 'recorded' || status === 'skipped' || status === 'waiting_approval';
    
    // Highlight step if it is running, OR if the job is paused and this is the current job step
    const isPausedAtThisStep = jobStatus === 'paused' && workflow?.job?.step === step;
    const active = status === 'running' || isPausedAtThisStep;
    const failedHere = status === 'failed';
    const dur = stepDurations.get(step);
    
    // Formatting step name
    let label = step.replace(/_/g, " ").replace(/\b\w/g, c => c.toUpperCase());
    let desc = '';
    let tasks: string[] = [];
    if (step.startsWith("code_backend_") || step.startsWith("code_frontend_")) {
      const type = step.startsWith("code_backend_") ? "Backend" : "Frontend";
      const idx = Number(step.substring(`code_${type.toLowerCase()}_`.length));
      label = `${type} Execution ${!isNaN(idx) ? idx + 1 : ""}`;

      const item = implementationItems?.find(i => i.stepId === step);
      if (item) {
        if (item.name) desc = item.name;
        if (item.tasks) tasks = item.tasks;
      }
    }

    return {
      label,
      desc,
      tasks,
      dur,
      icon: failedHere ? '✕' : done ? '✓' : active ? '●' : '',
      bg: failedHere ? 'rgba(239, 68, 68, 0.1)' : done ? 'rgba(16, 185, 129, 0.1)' : active ? 'rgba(59, 130, 246, 0.1)' : 'transparent',
      fg: failedHere ? '#ef4444' : done ? '#10b981' : active ? '#3b82f6' : 'rgba(148, 163, 184, 0.8)',
      ring: failedHere ? 'rgba(239, 68, 68, 0.25)' : done ? 'rgba(16, 185, 129, 0.25)' : active ? 'rgba(59, 130, 246, 0.25)' : 'rgba(148, 163, 184, 0.15)',
      weight: active || failedHere ? 600 : 400,
      color: failedHere ? '#ef4444' : active ? '#3b82f6' : done ? 'var(--foreground)' : 'var(--content-muted)',
      sub: failedHere ? 'failed' : active ? (isPausedAtThisStep ? 'paused' : 'in progress') : done ? '' : '',
      subC: failedHere ? '#ef4444' : active ? '#3b82f6' : 'transparent',
      done,
      active,
    };
  });

  const handleDeleteConfirm = async () => {
    if (confirm("Are you sure you want to delete this task?")) {
      const success = await deleteTask();
      if (success) router.push(`/projects/${projectID}`);
    }
  };

  return (
    <>
      <div className="bg-card border border-stroke/10 rounded-2xl p-5.5 shadow-sm hover:shadow-md transition-all duration-200">
        <div className="text-[10px] font-bold tracking-wider uppercase text-content-muted mb-3.5">Workflow Progress</div>
        <div className="flex flex-col gap-2">
          {phases.map((ph, idx) => (
            <div key={idx} className="flex items-start gap-3 py-2 border-b border-stroke/[0.03] last:border-none">
              <span className="inline-flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold shrink-0 border mt-0.5 transition-all" style={{
                borderColor: ph.ring,
                background: ph.bg,
                color: ph.fg
              }}>{ph.icon}</span>
              <div className="flex-1 flex flex-col min-w-0">
                <div className="flex justify-between items-center w-full">
                  <span className="text-[13px] font-semibold leading-snug" style={{ color: ph.color }}>{ph.label}</span>
                  <div className="flex items-center ml-2 shrink-0 gap-1.5">
                    {ph.sub && <span className="text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded" style={{ color: ph.subC, background: `${ph.subC}15` }}>{ph.sub}</span>}
                    {ph.dur && <span className="text-[10px] text-content-muted font-medium bg-slate-500/5 px-1 py-0.5 rounded">{ph.dur}</span>}
                  </div>
                </div>
                {ph.desc && (
                  <div className="text-[11px] text-content-muted mt-1 leading-tight line-clamp-2" title={ph.desc}>
                    {ph.desc}
                  </div>
                )}
                {ph.tasks && ph.tasks.length > 0 && (
                  <ul className="mt-2 space-y-1.5 pl-0.5">
                    {ph.tasks.map((t, tidx) => (
                      <li key={tidx} className="flex items-start gap-2 text-[11px] text-content-muted leading-relaxed">
                        <span className="shrink-0 mt-[4px]">
                          {ph.done ? (
                            <span className="text-emerald-500 font-bold text-[10px]">✓</span>
                          ) : ph.active ? (
                            <span className="w-1.5 h-1.5 rounded-full bg-blue-500 inline-block animate-pulse"></span>
                          ) : (
                            <span className="w-1.5 h-1.5 rounded-full bg-stroke/40 inline-block"></span>
                          )}
                        </span>
                        <span className={ph.done ? "line-through opacity-60" : ""}>{t}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="bg-card border border-stroke/10 rounded-2xl p-5.5 shadow-sm hover:shadow-md transition-all duration-200">
        <div className="text-[10px] font-bold tracking-wider uppercase text-content-muted mb-3.5">Details</div>
        <div className="flex flex-col gap-3 text-xs">
          <div className="flex justify-between items-center py-1 border-b border-stroke/5"><span className="text-content-muted">Status</span><span className="font-mono text-xs font-semibold px-2 py-0.5 rounded bg-slate-500/5" style={{ color: fg }}>{code}</span></div>
          <div className="flex justify-between items-center py-1 border-b border-stroke/5"><span className="text-content-muted">Priority</span><span className="font-bold text-rose-600 dark:text-rose-400 bg-rose-500/10 px-2 py-0.5 rounded border border-rose-500/25">P{task?.priority || 0}</span></div>
          <div className="flex justify-between items-center py-1 border-b border-stroke/5"><span className="text-content-muted">Agent</span><span className="font-semibold text-foreground">{task?.agent_id ? "Agentic Autonomous" : "Auto Mode"}</span></div>
          <div className="flex justify-between items-center py-1 border-b border-stroke/5"><span className="text-content-muted">Project ID</span><span className="font-mono text-xs text-foreground truncate max-w-[120px]" title={projectID}>{projectID}</span></div>
          <div className="flex justify-between items-center py-1"><span className="text-content-muted">Created</span><span className="font-semibold text-foreground">{task?.created_at ? new Date(task.created_at).toLocaleDateString() : "—"}</span></div>
        </div>
      </div>

      <div className="bg-card border border-rose-500/15 bg-rose-500/[0.01] rounded-2xl p-5.5 shadow-sm hover:shadow-md transition-all duration-200">
        <div className="text-[10px] font-bold tracking-wider uppercase text-rose-600 dark:text-rose-500 mb-3.5">Danger Zone</div>
        <div className="flex flex-col gap-2">
          {canCancel && (
            <button onClick={cancel} className="w-full px-3.5 py-2.5 rounded-xl border border-rose-500/20 bg-background/50 text-xs font-bold text-rose-600 cursor-pointer text-left hover:bg-rose-500/10 transition-all duration-150 flex items-center gap-2">
              ⊘ Close Task
            </button>
          )}
          <button onClick={handleDeleteConfirm} className="w-full px-3.5 py-2.5 rounded-xl border border-rose-500/20 bg-background/50 text-xs font-bold text-rose-600 cursor-pointer text-left hover:bg-rose-500/10 transition-all duration-150 flex items-center gap-2">
            🗑 Delete Task
          </button>
        </div>
      </div>
    </>
  );
}
