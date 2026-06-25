import type { Task, Agent } from "@/lib/types";

const activeStatuses = new Set([
  "context_loading",
  "analyzing",
  "running",
  "assigned",
  "planning",
  "coding",
  "reviewing",
  "fixing",
  "testing",
  "in_progress",
]);

const reviewStatuses = new Set(["spec_review", "human_review"]);
const failedStatuses = new Set(["failed", "blocked", "needs_changes", "changes_requested"]);

export const workflowStages = [
  { label: "Todo", statuses: ["todo"] },
  { label: "Analyze", statuses: ["context_loading", "analyzing", "planning"] },
  { label: "Spec Review", statuses: ["spec_review"] },
  { label: "Code", statuses: ["assigned", "in_progress", "running", "coding"] },
  { label: "Review/Fix", statuses: ["reviewing", "fixing"] },
  { label: "Test", statuses: ["testing"] },
  { label: "PR", statuses: ["pr_ready"] },
  { label: "Human Review", statuses: ["human_review"] },
  { label: "Merged", statuses: ["merged", "done", "completed"] },
];

export function isActiveTask(task: Task) {
  return activeStatuses.has(task.status);
}

export function needsReview(task: Task) {
  return reviewStatuses.has(task.status) || task.spec_status === "pending_review";
}

export function isFailedTask(task: Task) {
  return failedStatuses.has(task.status);
}

export function agentName(task: Task, agents: Agent[]) {
  if (!task.agent_id) return "Auto-assign";
  return agents.find((a) => a.id === task.agent_id)?.name ?? task.agent_id.slice(0, 8);
}

export const getRiskAssessment = (complexity: string, files: string[], riskDomains?: string[]) => {
  const fileCount = files.length;
  let hasMigration = false;
  let hasConfig = false;
  for (const f of files) {
    const lower = f.toLowerCase();
    if (lower.includes("migration/") || lower.includes(".sql")) hasMigration = true;
    if (lower.includes("config") || lower.includes(".env") || lower.includes("docker")) hasConfig = true;
  }

  // Check risk domains
  let hasHighRiskDomain = false;
  if (riskDomains && riskDomains.length > 0) {
    const highRisk = ["auth", "payment", "security", "infra", "rbac", "permission"];
    for (const d of riskDomains) {
      if (highRisk.includes(d.toLowerCase())) {
        hasHighRiskDomain = true;
      }
    }
  }

  if (hasMigration && complexity === "hard") {
    return { level: "critical", reason: "Database migration in a hard-complexity task requires careful review" };
  }
  if (hasHighRiskDomain && complexity === "hard") {
    return { level: "critical", reason: "Modifying high-risk domains in a hard-complexity task requires extreme caution" };
  }
  if (hasMigration) {
    return { level: "high", reason: "Contains database migration files" };
  }
  if (hasHighRiskDomain) {
    return { level: "high", reason: "Modifies high-risk security, authentication, or payment systems" };
  }
  if (complexity === "hard" || fileCount > 15) {
    return { level: "high", reason: `Hard complexity task affecting ${fileCount} files` };
  }
  if (hasConfig) {
    return { level: "medium", reason: "Modifies configuration or infrastructure files" };
  }
  if (complexity === "medium" || fileCount > 5) {
    return { level: "medium", reason: `Medium complexity task affecting ${fileCount} files` };
  }
  return { level: "low", reason: `Simple change affecting ${fileCount} files` };
};

export function deriveHydratedProjectStatus(doneTasks: number, totalTasks: number, hasHydratedCounts: boolean) {
  if (!hasHydratedCounts) return "loading";
  if (totalTasks === 0) return "idle";
  if (doneTasks === totalTasks) return "done";
  return "active";
}

export function isDoneStatus(status: string) {
  return ["done", "completed", "merged"].includes(status);
}

export function deriveProjectStatus(tasks: Task[]) {
  if (tasks.length === 0) return "idle";
  if (tasks.some(isFailedTask)) {
    return "blocked";
  }
  if (tasks.some((task) => isActiveTask(task) || task.status === "approved" || task.status === "queued")) {
    return "active";
  }
  if (tasks.every((task) => isDoneStatus(task.status))) return "done";
  return "idle";
}

export function workflowStageCounts(tasks: Task[]) {
  return workflowStages.map((stage) => ({
    ...stage,
    count: tasks.filter((task) => stage.statuses.includes(task.status)).length,
  }));
}

export function latestActivity(tasks: Task[], fallback: string) {
  return tasks.reduce((latest, task) => {
    return new Date(task.updated_at).getTime() > new Date(latest).getTime() ? task.updated_at : latest;
  }, fallback);
}

export function splitTaskDescription(description: string) {
  const markers = [
    "\n\nRequested changes:\n",
    "\n\nClarification:\n",
  ];

  let splitAt = -1;
  let marker = "";
  for (const candidate of markers) {
    const idx = description.indexOf(candidate);
    if (idx !== -1 && (splitAt === -1 || idx < splitAt)) {
      splitAt = idx;
      marker = candidate;
    }
  }

  if (splitAt === -1) {
    return { body: description.trim(), context: "" };
  }

  return {
    body: description.slice(0, splitAt).trim(),
    context: description.slice(splitAt + marker.length).trim(),
  };
}
