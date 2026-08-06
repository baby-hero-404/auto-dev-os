import type { Task, TaskSpecStatus, TaskStatus, WorkflowJob } from "@/lib/types";

/**
 * The one canonical enumeration of CLI-spec-first flow step ids, in order.
 * Includes both the single-track `cli_implement` shape (workflow.CLISpecFirstWorkflow)
 * and the dual-agent parallel-track shape (workflow.CLISpecFirstParallelWorkflow,
 * `cli_implement_backend`/`cli_implement_frontend` joined by the shared `merge` step)
 * — a given task's checkpoints only ever populate one of the two shapes.
 */
export const CLI_STEPS = [
  "cli_analyze",
  "cli_spec",
  "cli_implement",
  "cli_implement_backend",
  "cli_implement_frontend",
  "merge",
  "cross_review",
  "cli_mr",
] as const;

export type BadgeVariant =
  | "neutral"
  | "accent"
  | "success"
  | "warning"
  | "danger"
  | "info"
  | "purple"
  | "cyan"
  | "orange"
  | "violet"
  | "indigo"
  | "teal"
  | "yellow";

export interface StatusBadge {
  variant: BadgeVariant;
  label: string;
  bg?: string;
  fg?: string;
  group?: string;
  chartColor?: string;
}

export const TASK_STATUS_BADGES: Record<TaskStatus, StatusBadge> = {
  todo: { variant: "neutral", label: "Todo", bg: "var(--surface)", fg: "var(--content-muted)", group: "Preparation", chartColor: "#64748b" },
  context_loading: { variant: "indigo", label: "Loading Context", bg: "#e0efff", fg: "#005bb8", group: "Preparation", chartColor: "#3b82f6" },
  analyzing: { variant: "info", label: "Analyzing", bg: "#e0efff", fg: "#005bb8", group: "Preparation", chartColor: "#f59e0b" },
  spec_review: { variant: "purple", label: "Spec Review", bg: "#fef3c6", fg: "#795800", group: "Preparation · Gate", chartColor: "#a78bfa" },
  planning_split: { variant: "warning", label: "Split Review", bg: "#fef3c6", fg: "#b45309", group: "Preparation · Gate", chartColor: "#f59e0b" },
  coding: { variant: "cyan", label: "Coding", bg: "#e0efff", fg: "#005bb8", group: "Execution", chartColor: "#22c55e" },
  reviewing: { variant: "violet", label: "Reviewing", bg: "#f3e8ff", fg: "#7f22fe", group: "Execution", chartColor: "#06b6d4" },
  fixing: { variant: "orange", label: "Fixing", bg: "#fff1e0", fg: "#b75000", group: "Execution", chartColor: "#fb923c" },
  testing: { variant: "teal", label: "Testing", bg: "#e0efff", fg: "#005bb8", group: "Execution", chartColor: "#14b8a6" },
  pr_ready: { variant: "purple", label: "PR Ready", bg: "#d9f5e7", fg: "#007956", group: "Finalization", chartColor: "#10b981" },
  human_review: { variant: "yellow", label: "Human Review", bg: "#fef3c6", fg: "#795800", group: "Finalization · Gate", chartColor: "#e879f9" },
  merged: { variant: "success", label: "Merged", bg: "#e6f4ea", fg: "#00590e", group: "Finalization", chartColor: "#34d399" },
  failed: { variant: "danger", label: "Failed", bg: "#ffe2e2", fg: "#bf000f", group: "Finalization", chartColor: "#ef4444" },
  blocked: { variant: "warning", label: "Blocked", bg: "#fef3c6", fg: "#b45309", group: "Execution · Blocked", chartColor: "#d97706" },
};

const SPEC_STATUS_BADGES: Record<TaskSpecStatus, StatusBadge> = {
  none: { variant: "neutral", label: "None" },
  draft: { variant: "info", label: "Draft" },
  pending_review: { variant: "warning", label: "Pending Review" },
  changes_requested: { variant: "danger", label: "Changes Requested" },
  clarification_required: { variant: "orange", label: "Clarification Required" },
  approved: { variant: "success", label: "Approved" },
  auto_approved: { variant: "success", label: "Auto Approved" },
  ready_with_warnings: { variant: "yellow", label: "Ready (Warnings)" },
};

export function getTaskStatusBadge(status: TaskStatus): StatusBadge {
  return TASK_STATUS_BADGES[status];
}

export function getSpecStatusBadge(status: TaskSpecStatus): StatusBadge {
  return SPEC_STATUS_BADGES[status];
}

const TERMINAL_STATUSES = new Set<TaskStatus>(["merged", "failed"]);
const ACTIVE_STATUSES = new Set<TaskStatus>([
  "context_loading",
  "analyzing",
  "coding",
  "reviewing",
  "fixing",
  "testing",
]);

export function isTerminalStatus(status: TaskStatus): boolean {
  return TERMINAL_STATUSES.has(status);
}

export function isActiveStatus(status: TaskStatus): boolean {
  return ACTIVE_STATUSES.has(status);
}

export function isTodoStatus(status: TaskStatus): boolean {
  return status === "todo";
}

export function isPreparationStatus(status: TaskStatus): boolean {
  return status === "context_loading" || status === "analyzing";
}

export function isExecutionStatus(status: TaskStatus): boolean {
  return status === "coding" || status === "testing" || status === "fixing";
}

export function isReviewingStatus(status: TaskStatus): boolean {
  return status === "reviewing";
}

export function isFailedStatus(status: TaskStatus): boolean {
  return status === "failed";
}

export function isMergedStatus(status: TaskStatus): boolean {
  return status === "merged";
}

export function needsReview(task: Pick<Task, "status" | "spec_status">): boolean {
  return (
    task.status === "spec_review" ||
    task.status === "human_review" ||
    task.spec_status === "pending_review" ||
    task.spec_status === "clarification_required"
  );
}

export function isPendingSpecReview(task: Pick<Task, "status" | "spec_status">): boolean {
  return (
    task.status === "spec_review" ||
    task.spec_status === "pending_review" ||
    task.spec_status === "clarification_required"
  );
}

export function isPrReadyStatus(status?: TaskStatus): boolean {
  return status === "pr_ready";
}

const SPEC_GATE_STATUSES = new Set<TaskSpecStatus>([
  "pending_review",
  "changes_requested",
  "clarification_required",
]);

export function canResumeTask(task: Pick<Task, "status" | "spec_status">): boolean {
  return (
    task.status !== "pr_ready" &&
    task.status !== "human_review" &&
    task.status !== "merged" &&
    !SPEC_GATE_STATUSES.has(task.spec_status)
  );
}

export function showAnalyzeAction(task: Pick<Task, "status" | "spec_status">): boolean {
  return (task.status === "todo" || task.status === "failed") && !isExecutionReady(task);
}

export function isExecutionReady(task: Pick<Task, "status" | "spec_status">): boolean {
  return (
    (task.spec_status === "auto_approved" || task.spec_status === "approved") &&
    (task.status === "todo" || task.status === "failed")
  );
}

export function isEffectivelyFailed(
  task: Pick<Task, "status">,
  job?: Pick<WorkflowJob, "status" | "last_error"> | null
): boolean {
  if (task.status === "failed") return true;
  if (job?.status === "failed") return true;
  if (job?.status === "paused" && job.last_error != null && job.last_error !== "") {
    const isPauseReason = job.last_error.includes("workflow paused for human spec review") || 
                          job.last_error.includes("workflow paused for human task clarification") ||
                          job.last_error.includes("workflow paused by user") ||
                          job.last_error.includes("workflow waiting approval");
    if (!isPauseReason) {
      return true;
    }
  }
  return false;
}
