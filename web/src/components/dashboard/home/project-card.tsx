import Link from "next/link";
import { useRef } from "react";
import useSWR from "swr";
import { Bot, CheckCircle2, GitBranch, Clock } from "lucide-react";
import type { Project } from "@/lib/types";
import { api } from "@/lib/api";
import { useIsNearViewport } from "@/lib/hooks/use-is-near-viewport";
import { formatRelativeTime } from "@/lib/utils/time";
import {
  deriveHydratedProjectStatus,
  deriveProjectStatus,
  isDoneStatus,
  latestActivity,
} from "@/lib/utils/task-utils";

export interface ProjectCardProps {
  project: Project;
  token: string;
}

function CardStat({
  icon: Icon,
  label,
  value,
}: {
  icon: typeof GitBranch;
  label: string;
  value: string;
}) {
  return (
    <div className="rounded-md border border-stroke bg-background/50 p-2">
      <div className="mb-1 flex items-center gap-1.5">
        <Icon size={13} className="text-brand-primary" />
        <span>{label}</span>
      </div>
      <div className="font-mono text-sm font-semibold text-foreground">{value}</div>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    loading: "border-stroke bg-surface text-content-muted",
    idle: "border-stroke bg-surface text-content-muted",
    active: "border-cyan-400/20 bg-cyan-400/10 text-cyan-700 dark:text-cyan-300",
    blocked: "border-red-400/20 bg-red-400/10 text-red-700 dark:text-red-300",
    done: "border-emerald-400/20 bg-emerald-400/10 text-emerald-700 dark:text-emerald-300",
  };

  return (
    <span className={`shrink-0 rounded border px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider ${styles[status] || styles.idle}`}>
      {status}
    </span>
  );
}

export function ProjectCard({ project, token }: ProjectCardProps) {
  const cardRef = useRef<HTMLAnchorElement>(null);
  const isVisible = useIsNearViewport(cardRef);
  const hasHydratedCounts =
    project.repositories_count !== undefined ||
    project.agents_count !== undefined ||
    project.tasks_total_count !== undefined ||
    project.tasks_done_count !== undefined;
  
  const { data: meta } = useSWR(
    token && isVisible && !hasHydratedCounts ? ["project-card-meta", project.id] : null,
    async () => {
      const [repositories, agents, tasks] = await Promise.allSettled([
        api.listRepositories(project.id, token),
        api.listAgents(project.id, token),
        api.listTasks(project.id, token),
      ]);

      return {
        repositories: repositories.status === "fulfilled" ? repositories.value : null,
        agents: agents.status === "fulfilled" ? agents.value : null,
        tasks: tasks.status === "fulfilled" ? tasks.value : null,
      };
    },
  );

  const tasks = meta?.tasks;
  const totalTasks = project.tasks_total_count ?? tasks?.length ?? 0;
  const doneTasks = project.tasks_done_count ?? tasks?.filter((task) => isDoneStatus(task.status)).length ?? 0;
  const repositoriesCount = project.repositories_count ?? meta?.repositories?.length;
  const agentsCount = project.agents_count ?? meta?.agents?.length;
  const progress = totalTasks === 0 ? 0 : Math.round((doneTasks / totalTasks) * 100);
  const status = tasks ? deriveProjectStatus(tasks) : deriveHydratedProjectStatus(doneTasks, totalTasks, hasHydratedCounts);
  const lastActivity = tasks ? latestActivity(tasks, project.updated_at) : project.updated_at;

  return (
    <Link
      ref={cardRef}
      href={`/projects/${project.id}`}
      className="group glow-on-hover flex min-h-[230px] flex-col justify-between rounded-lg border border-stroke bg-card p-5 transition hover:border-brand-primary/40"
    >
      <div>
        <div className="mb-4 flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="truncate font-mono text-lg font-semibold text-foreground transition duration-150 group-hover:text-brand-primary">
              {project.name}
            </h3>
            <p className="mt-2 line-clamp-2 text-sm text-content-muted">
              {project.description || "No project description provided."}
            </p>
          </div>
          <StatusBadge status={status} />
        </div>

        <div className="grid grid-cols-3 gap-2 text-xs text-content-muted">
          <CardStat icon={GitBranch} label="Repos" value={repositoriesCount !== undefined ? repositoriesCount.toString() : "--"} />
          <CardStat icon={Bot} label="Agents" value={agentsCount !== undefined ? agentsCount.toString() : "--"} />
          <CardStat icon={CheckCircle2} label="Tasks" value={hasHydratedCounts || tasks ? `${doneTasks}/${totalTasks}` : "--"} />
        </div>
      </div>

      <div className="mt-5">
        <div className="h-1.5 overflow-hidden rounded-full bg-background">
          <div className="h-full rounded-full bg-brand-primary transition-all" style={{ width: `${progress}%` }} />
        </div>
        <div className="mt-3 flex items-center justify-between gap-3 font-mono text-xs text-content-muted">
          <span>{progress}% complete</span>
          <span className="inline-flex min-w-0 items-center gap-1 text-right">
            <Clock size={12} />
            <span className="truncate">{formatRelativeTime(lastActivity)}</span>
          </span>
        </div>
      </div>
    </Link>
  );
}

export function ProjectCardsSkeleton() {
  return (
    <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {[0, 1, 2].map((item) => (
        <div key={item} className="min-h-[230px] rounded-lg border border-stroke bg-card p-5">
          <div className="mb-4 flex items-start justify-between gap-3">
            <div className="min-w-0 flex-1 space-y-2">
               <div className="skeleton-shimmer h-6 w-40 rounded" />
               <div className="skeleton-shimmer h-4 w-full max-w-[260px] rounded" />
            </div>
            <div className="skeleton-shimmer h-5 w-16 rounded" />
          </div>
          <div className="grid grid-cols-3 gap-2">
            <div className="skeleton-shimmer h-14 rounded-md" />
            <div className="skeleton-shimmer h-14 rounded-md" />
            <div className="skeleton-shimmer h-14 rounded-md" />
          </div>
          <div className="mt-5">
            <div className="skeleton-shimmer h-1.5 rounded-full" />
            <div className="mt-3 flex items-center justify-between">
               <div className="skeleton-shimmer h-3 w-20 rounded" />
               <div className="skeleton-shimmer h-3 w-24 rounded" />
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
