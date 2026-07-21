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
    const done = status === 'success' || status === 'recorded' || status === 'skipped';
    
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
      bg: failedHere ? '#ffe2e2' : done ? '#e6f4ea' : active ? '#e0efff' : 'transparent',
      fg: failedHere ? '#bf000f' : done ? '#00590e' : active ? '#005bb8' : 'var(--content-muted)',
      ring: failedHere ? '#ffa3a3' : done ? '#a4f4cf' : active ? '#90c5ff' : 'var(--stroke)',
      weight: active || failedHere ? 600 : 400,
      color: failedHere ? '#bf000f' : active ? '#005bb8' : done ? 'var(--foreground)' : 'var(--content-muted)',
      sub: failedHere ? 'failed' : active ? (isPausedAtThisStep ? 'paused' : 'in progress') : done ? '' : '',
      subC: failedHere ? '#bf000f' : active ? '#005bb8' : 'transparent',
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
      <div className="bg-card border border-stroke rounded-xl px-5 py-4.5">
        <div className="text-xs font-semibold tracking-wider uppercase text-content-muted mb-3">Workflow</div>
        {phases.map((ph, idx) => (
          <div key={idx} className="flex items-start gap-2.5 py-1.5">
            <span className="inline-flex items-center justify-center w-5 h-5 rounded-full text-[10px] font-bold shrink-0 border-[1.5px] mt-0.5" style={{
              borderColor: ph.ring,
              background: ph.bg,
              color: ph.fg
            }}>{ph.icon}</span>
            <div className="flex-1 flex flex-col min-w-0">
              <div className="flex justify-between items-center w-full">
                <span className="text-[13px]" style={{ fontWeight: ph.weight, color: ph.color }}>{ph.label}</span>
                <div className="flex items-center ml-2 shrink-0 gap-1.5">
                  {ph.sub && <span className="text-[11px] font-medium" style={{ color: ph.subC }}>{ph.sub}</span>}
                  {ph.dur && <span className="text-[10px] text-content-muted font-medium">{ph.dur}</span>}
                </div>
              </div>
              {ph.desc && (
                <div className="text-[11px] text-content-muted mt-0.5 leading-tight line-clamp-2" title={ph.desc}>
                  {ph.desc}
                </div>
              )}
              {ph.tasks && ph.tasks.length > 0 && (
                <ul className="mt-1.5 space-y-1">
                  {ph.tasks.map((t, tidx) => (
                    <li key={tidx} className="flex items-start gap-1.5 text-[10.5px] text-content-muted leading-tight">
                      <span className="shrink-0 mt-[1.5px]">
                        {ph.done ? (
                          <span className="text-[#00590e] font-bold text-[9px]">✓</span>
                        ) : ph.active ? (
                          <span className="w-1 h-1 rounded-full bg-[#005bb8] inline-block mb-[1px]"></span>
                        ) : (
                          <span className="w-1 h-1 rounded-full bg-stroke inline-block mb-[1px]"></span>
                        )}
                      </span>
                      <span className={ph.done ? "line-through opacity-70" : ""}>{t}</span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </div>
        ))}
      </div>

      <div className="bg-card border border-stroke rounded-xl px-5 py-4.5">
        <div className="text-xs font-semibold tracking-wider uppercase text-content-muted mb-3">Details</div>
        <div className="flex flex-col gap-2.5 text-[13px]">
          <div className="flex justify-between"><span className="text-content-muted">Status</span><span className="font-mono text-xs font-semibold" style={{ color: fg }}>{code}</span></div>
          <div className="flex justify-between"><span className="text-content-muted">Priority</span><span className="font-medium text-danger">P{task?.priority || 0}</span></div>
          <div className="flex justify-between"><span className="text-content-muted">Agent</span><span className="font-medium text-foreground">{task?.agent_id ? "Agent" : "Auto"}</span></div>
          <div className="flex justify-between"><span className="text-content-muted">Project</span><span className="font-medium text-foreground truncate max-w-[120px]">{projectID}</span></div>
          <div className="flex justify-between"><span className="text-content-muted">Created</span><span className="font-medium text-foreground">{task?.created_at ? new Date(task.created_at).toLocaleDateString() : "Unknown"}</span></div>
        </div>
      </div>

      <div className="bg-card border border-danger/30 rounded-xl px-5 py-4.5">
        <div className="text-xs font-semibold tracking-wider uppercase text-danger mb-2.5">Danger Zone</div>
        <div className="flex flex-col gap-2">
          {canCancel && (
            <button onClick={cancel} className="px-3 py-2 rounded-lg border border-stroke bg-card text-[13px] font-medium text-danger cursor-pointer text-left hover:bg-danger/10 transition">
              ⊘ Close Task
            </button>
          )}
          <button onClick={handleDeleteConfirm} className="px-3 py-2 rounded-lg border border-stroke bg-card text-[13px] font-medium text-danger cursor-pointer text-left hover:bg-danger/10 transition">
            🗑 Delete Task
          </button>
        </div>
      </div>
    </>
  );
}
