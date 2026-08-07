/** Mirrors server/pkg/models TaskEvent — see design.md's TaskEvent Model (Go) section. */
export type TaskEvent = {
  id: string;
  task_id: string;
  sequence_number: number;
  type: TaskEventType;
  schema_version: number;
  payload: Record<string, unknown>;
  artifact_id?: string;
  size_bytes: number;
  created_at: string;
};

export type TaskEventType =
  | "task.started"
  | "task.completed"
  | "task.error"
  | "status.changed"
  | "agent.reasoning_summary"
  | "agent.plan"
  | "agent.message"
  | "tool.started"
  | "tool.finished"
  | "file.changed"
  | "command.started"
  | "command.finished"
  | "test.result";

export type TaskStartedPayload = { step: string };
export type TaskCompletedPayload = { step: string; duration_ms: number };
export type TaskErrorPayload = { step: string; error: string; is_retryable: boolean };
export type StatusChangedPayload = { from: string; to: string };
export type AgentReasoningSummaryPayload = { summary: string };
export type AgentPlanPayload = { steps: string[] };
export type AgentMessagePayload = { text: string };
export type ToolStartedPayload = { tool: string; input: string };
export type ToolFinishedPayload = { tool: string; output: string; duration_ms: number; success: boolean };
export type FileChangedPayload = { path: string; additions: number; deletions: number };
export type CommandStartedPayload = { command: string; cwd: string };
export type CommandFinishedPayload = { command: string; exit_code: number; stdout_tail: string; stderr_tail: string };
export type TestResultPayload = { passed: number; failed: number; skipped: number; details?: string };
