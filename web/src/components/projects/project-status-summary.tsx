import { Bot, GitPullRequest, Radio, TriangleAlert } from "lucide-react";
import type { Task, Agent } from "@/lib/types";
import { isActiveTask, isFailedTask, needsReview, isDoneStatus, workflowStageCounts } from "@/lib/utils/task-utils";

interface ProjectStatusSummaryProps {
  tasks: Task[];
  projectAgents: Agent[];
}

export function ProjectStatusSummary({ tasks, projectAgents }: ProjectStatusSummaryProps) {
  const activeTasks = tasks.filter(t => isActiveTask(t)).length;
  const reviewTasks = tasks.filter(needsReview).length;
  const failedTasks = tasks.filter(isFailedTask).length;
  const completedTasks = tasks.filter((task) => isDoneStatus(task.status)).length;
  const completion = tasks.length > 0 ? Math.round((completedTasks / tasks.length) * 100) : 0;

  const stats = [
    {
      label: "Active Tasks",
      value: activeTasks,
      icon: Radio,
      color: "text-success",
      borderColor: "border-success/20",
      bgColor: "bg-success/5",
    },
    {
      label: "Needs Review",
      value: reviewTasks,
      icon: GitPullRequest,
      color: "text-warning",
      borderColor: "border-warning/20",
      bgColor: "bg-warning/5",
    },
    {
      label: "Failed",
      value: failedTasks,
      icon: TriangleAlert,
      color: "text-danger",
      borderColor: "border-danger/20",
      bgColor: "bg-danger/5",
    },
    {
      label: "Agents Assigned",
      value: projectAgents.length,
      icon: Bot,
      color: "text-info",
      borderColor: "border-info/20",
      bgColor: "bg-info/5",
    },
  ];

  return (
    <div className="space-y-4">
      <section className="grid gap-3 xl:grid-cols-[280px_1fr]">
        <div className="rounded-lg border border-stroke bg-card p-4">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-xs font-medium text-content-muted">Flow completion</div>
              <div className="mt-1 font-mono text-2xl font-semibold text-foreground">{completion}%</div>
            </div>
            <div className="text-right text-xs text-content-muted">
              <div><span className="font-mono text-foreground">{completedTasks}</span> merged</div>
              <div><span className="font-mono text-foreground">{tasks.length}</span> total</div>
            </div>
          </div>
          <div className="mt-4 h-2 overflow-hidden rounded-full bg-surface">
            <div
              className="h-full rounded-full bg-brand-primary transition-all"
              style={{ width: `${completion}%` }}
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {stats.map((stat) => (
            <div
              key={stat.label}
              className={`flex min-h-24 flex-col justify-between rounded-lg border ${stat.borderColor} ${stat.bgColor} p-4 transition duration-200 hover:border-opacity-80`}
            >
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs font-medium text-content-muted">{stat.label}</span>
                <stat.icon size={16} className={stat.color} />
              </div>
              <span className={`mt-2 font-mono text-2xl font-semibold tracking-tight ${stat.color}`}>{stat.value}</span>
            </div>
          ))}
        </div>
      </section>

      <WorkflowStageStrip tasks={tasks} />
    </div>
  );
}

function WorkflowStageStrip({ tasks }: { tasks: Task[] }) {
  const stages = workflowStageCounts(tasks);

  return (
    <div className="overflow-x-auto pb-1">
      <div className="flex min-w-max gap-2">
        {stages.map((stage) => {
          const hasTasks = stage.count > 0;
          return (
            <div
              key={stage.label}
              className={`flex min-w-28 items-center justify-between gap-3 rounded-md border px-3 py-2 text-xs transition ${
                hasTasks
                  ? "border-brand-primary/30 bg-brand-primary-muted text-foreground"
                  : "border-stroke bg-surface text-content-muted"
              }`}
            >
              <span className="font-medium">{stage.label}</span>
              <span className="font-mono font-semibold">{stage.count}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}
