import type { Task, Agent } from "@/lib/types";
import { isActiveTask } from "@/lib/utils/task-utils";

interface ProjectStatusSummaryProps {
  tasks: Task[];
  projectAgents: Agent[];
}

export function ProjectStatusSummary({ tasks, projectAgents }: ProjectStatusSummaryProps) {
  const activeTasks = tasks.filter(t => isActiveTask(t)).length;
  const needsReview = tasks.filter(
    t => t.status === "spec_review" || t.status === "human_review" || t.spec_status === "pending_review"
  ).length;
  const failedTasks = tasks.filter(t => t.status === "failed").length;

  const stats = [
    {
      label: "Active Tasks",
      value: activeTasks,
      color: "text-emerald-500 dark:text-emerald-400",
      borderColor: "border-emerald-500/20",
      bgColor: "bg-emerald-500/5",
    },
    {
      label: "Needs Review",
      value: needsReview,
      color: "text-amber-500 dark:text-amber-400",
      borderColor: "border-amber-500/20",
      bgColor: "bg-amber-500/5",
    },
    {
      label: "Failed",
      value: failedTasks,
      color: "text-rose-500 dark:text-rose-400",
      borderColor: "border-rose-500/20",
      bgColor: "bg-rose-500/5",
    },
    {
      label: "Agents Assigned",
      value: projectAgents.length,
      color: "text-blue-500 dark:text-blue-400",
      borderColor: "border-blue-500/20",
      bgColor: "bg-blue-500/5",
    },
  ];

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
      {stats.map((stat) => (
        <div
          key={stat.label}
          className={`flex flex-col justify-between rounded-lg border ${stat.borderColor} ${stat.bgColor} p-4 transition duration-200 hover:scale-[1.01]`}
        >
          <span className="text-xs font-medium text-content-muted">{stat.label}</span>
          <span className={`mt-2 font-sans text-2xl font-bold tracking-tight ${stat.color}`}>
            {stat.value}
          </span>
        </div>
      ))}
    </div>
  );
}
