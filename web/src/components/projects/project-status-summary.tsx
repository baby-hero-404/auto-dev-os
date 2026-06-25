import { Bot, GitPullRequest, Radio, TriangleAlert } from "lucide-react";
import type { Task, Agent } from "@/lib/types";
import { isActiveTask, isFailedTask, needsReview, isDoneStatus } from "@/lib/utils/task-utils";

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
      color: "text-emerald-500 dark:text-emerald-400",
      borderColor: "border-emerald-500/20",
      bgColor: "bg-emerald-500/5",
    },
    {
      label: "Needs Review",
      value: reviewTasks,
      icon: GitPullRequest,
      color: "text-amber-500 dark:text-amber-400",
      borderColor: "border-amber-500/20",
      bgColor: "bg-amber-500/5",
    },
    {
      label: "Failed",
      value: failedTasks,
      icon: TriangleAlert,
      color: "text-rose-500 dark:text-rose-400",
      borderColor: "border-rose-500/20",
      bgColor: "bg-rose-500/5",
    },
    {
      label: "Agents Assigned",
      value: projectAgents.length,
      icon: Bot,
      color: "text-blue-500 dark:text-blue-400",
      borderColor: "border-blue-500/20",
      bgColor: "bg-blue-500/5",
    },
  ];

  return (
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
  );
}
