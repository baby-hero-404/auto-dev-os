"use client";

import { createContext, useContext, useState, useMemo, useCallback } from "react";
import type { Task, WorkflowStatus, WorkflowArtifact, TaskAnalysis, AffectedFile, ExecutionUnit, WorkflowCheckpoint } from "@/lib/types";
import { getRiskAssessment, splitTaskDescription } from "@/lib/utils/task-utils";
import { parseUnifiedDiff, type ParsedFileDiff } from "@/components/projects/task-diff-viewer";
import { useTaskWorkflow } from "@/lib/hooks/use-task-workflow";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import type { RealtimeLog } from "@/lib/store/use-realtime-log-store";

export interface ImplementationItem {
  id: string;              // execution_unit.id
  name: string;            // execution_unit title/objective
  description: string;     // human-readable summary
  status: 'done' | 'running' | 'pending';
  affectedFiles: string[];
  stepId: string;          // e.g. "code_backend_2"
  checkpointExists: boolean;
  tasks?: string[];        // Fine-grained tasks mapped to this execution unit
}

export function deriveImplementationItems(
  analysis: Partial<TaskAnalysis> | undefined,
  checkpoints: WorkflowCheckpoint[] | undefined,
  currentStep: string | undefined
): ImplementationItem[] {
  if (!analysis?.execution_units || analysis.execution_units.length === 0) {
    return [];
  }

  const completedSteps = new Set<string>();
  const runningSteps = new Set<string>();
  
  if (checkpoints) {
    for (const cp of checkpoints) {
      if (!cp.step.startsWith("code_")) continue;
      const status = cp.state?.status;
      if (status === "success" || status === "recorded" || status === "skipped") {
        completedSteps.add(cp.step);
      } else if (status === "running") {
        runningSteps.add(cp.step);
      }
    }
  }

  if (currentStep && currentStep.startsWith("code_")) {
    runningSteps.add(currentStep);
  }

  let backendIdx = 0;
  let frontendIdx = 0;

  return analysis.execution_units.map((unit) => {
    const rawAgent = unit.execution_profile?.agent?.toLowerCase() || "backend";
    const role = (rawAgent === "frontend" || rawAgent === "fe") ? "frontend" : "backend";
    
    let idx = 0;
    if (role === "frontend") {
      idx = frontendIdx;
      frontendIdx++;
    } else {
      idx = backendIdx;
      backendIdx++;
    }

    const stepId = `code_${role}_${idx}`;
    const isDone = completedSteps.has(stepId);
    const isRunning = !isDone && runningSteps.has(stepId);
    
    const name = (unit as any).title || (unit as any).intent?.capability || unit.objective || `Unit ${idx}`;
    const description = (unit as any).description || unit.objective || '';
    const affectedFiles = (unit as any).affected_files || [];

    return {
      id: unit.id,
      name,
      description,
      status: isDone ? 'done' : isRunning ? 'running' : 'pending',
      affectedFiles,
      stepId,
      checkpointExists: isDone,
      tasks: (unit as any).tasks || [],
    };
  });
}

// Enums & Constants for Workflow Steps to avoid typos
export const WORKFLOW_STEPS = {
  CONTEXT_LOAD: "context_load",
  ANALYZE: "analyze",
  PLAN: "plan",
  CODE_BACKEND: "code_backend",
  CODE_FRONTEND: "code_frontend",
  MERGE: "merge",
  REVIEW: "review",
  FIX: "fix",
  TEST: "test",
  PR: "pr",
} as const;

export const EASY_STEPS = [
  WORKFLOW_STEPS.CONTEXT_LOAD,
  WORKFLOW_STEPS.ANALYZE,
  WORKFLOW_STEPS.CODE_BACKEND,
  WORKFLOW_STEPS.TEST,
  WORKFLOW_STEPS.PR,
];

// Helper functions for parsing
export function isAffectedFile(x: unknown): x is AffectedFile {
  return typeof x === "object" && x !== null && "file" in x && typeof (x as Record<string, unknown>).file === "string";
}

function parseTasksMD(tasksMD: string | undefined): { backend: string[]; frontend: string[] } {
  const result = { backend: [] as string[], frontend: [] as string[] };
  if (!tasksMD || !tasksMD.trim()) {
    return result;
  }

  const frontendSignals = ["frontend", "ui", "component", "page", "view", "style", "css", "layout", "giao diện", "giao dien"];
  const backendSignals = ["backend", "server", "api", "database", "db", "migration", "model", "service", "handler", "cơ sở dữ liệu", "co so du lieu"];

  function classifyHeading(heading: string): "backend" | "frontend" {
    const lower = heading.toLowerCase();
    for (const signal of frontendSignals) {
      if (lower.includes(signal)) return "frontend";
    }
    for (const signal of backendSignals) {
      if (lower.includes(signal)) return "backend";
    }
    return "backend";
  }

  function isCheckboxLine(line: string): boolean {
    return line.startsWith("- [ ]") || line.startsWith("- [x]") || line.startsWith("- [X]");
  }

  function extractCheckboxText(line: string): string {
    for (const prefix of ["- [ ] ", "- [x] ", "- [X] "]) {
      if (line.startsWith(prefix)) {
        return line.substring(prefix.length).trim();
      }
    }
    if (line.length > 6) {
      return line.substring(6).trim();
    }
    return "";
  }

  const lines = tasksMD.split("\n");
  const sections: Array<{ heading: string; role: "backend" | "frontend"; items: string[] }> = [];
  let currentSection: { heading: string; role: "backend" | "frontend"; items: string[] } | null = null;

  for (const line of lines) {
    const trimmed = line.trim();

    if (trimmed.startsWith("## ")) {
      if (currentSection && currentSection.items.length > 0) {
        sections.push(currentSection);
      }
      const heading = trimmed.substring(3).trim();
      const role = classifyHeading(heading);
      currentSection = {
        heading: trimmed,
        role: role,
        items: [],
      };
      continue;
    }

    if (isCheckboxLine(trimmed) && currentSection) {
      currentSection.items.push(trimmed);
    }
  }

  if (currentSection && currentSection.items.length > 0) {
    sections.push(currentSection);
  }

  for (const sec of sections) {
    const combined = [sec.heading, ...sec.items].join("\n");
    result[sec.role].push(combined);
  }

  if (result.backend.length === 0 && result.frontend.length === 0) {
    let currentRole: "backend" | "frontend" | null = null;
    for (const line of lines) {
      const trimmed = line.trim();
      if (trimmed.startsWith("## ")) {
        const heading = trimmed.substring(3).trim();
        currentRole = classifyHeading(heading);
        continue;
      }
      if (isCheckboxLine(trimmed) && currentRole) {
        const item = extractCheckboxText(trimmed);
        if (item) {
          result[currentRole].push(item);
        }
      }
    }
  }

  return result;
}

export function formatStepName(step: string, analysisData?: Partial<TaskAnalysis>): string {
  if (step.startsWith("code_backend_")) {
    const parsedIdx = Number(step.substring("code_backend_".length));
    if (!isNaN(parsedIdx) && analysisData?.execution_units) {
      const beUnits = analysisData.execution_units.filter(
        (unit: ExecutionUnit) => (unit.execution_profile?.agent?.toLowerCase() || "backend") !== "frontend" && (unit.execution_profile?.agent?.toLowerCase() || "backend") !== "fe"
      );
      if (beUnits[parsedIdx]?.objective) {
        return beUnits[parsedIdx].objective;
      }
    }
    const idx = isNaN(parsedIdx) ? 1 : parsedIdx + 1;
    return `be subtask ${idx}`;
  }
  if (step.startsWith("code_frontend_")) {
    const parsedIdx = Number(step.substring("code_frontend_".length));
    if (!isNaN(parsedIdx) && analysisData?.execution_units) {
      const feUnits = analysisData.execution_units.filter(
        (unit: ExecutionUnit) => (unit.execution_profile?.agent?.toLowerCase() || "backend") === "frontend" || (unit.execution_profile?.agent?.toLowerCase() || "backend") === "fe"
      );
      if (feUnits[parsedIdx]?.objective) {
        return feUnits[parsedIdx].objective;
      }
    }
    const idx = isNaN(parsedIdx) ? 1 : parsedIdx + 1;
    return `fe subtask ${idx}`;
  }
  return step.replace(/_/g, " ");
}

export function getStepDescription(step: string, analysisData?: Partial<TaskAnalysis>): string {
  if (step === WORKFLOW_STEPS.CONTEXT_LOAD) return "Load codebase structure, dependencies, and code patterns.";
  if (step === WORKFLOW_STEPS.ANALYZE) return "Analyze the request requirements and suggest high-level steps.";
  if (step === WORKFLOW_STEPS.PLAN) return "Break down the task into specific, sequential subtasks.";
  if (step.startsWith("code_backend_")) {
    const parsedIdx = Number(step.substring("code_backend_".length));
    if (!isNaN(parsedIdx) && analysisData?.execution_units) {
      const beUnits = analysisData.execution_units.filter(
        (unit: ExecutionUnit) => (unit.execution_profile?.agent?.toLowerCase() || "backend") !== "frontend" && (unit.execution_profile?.agent?.toLowerCase() || "backend") !== "fe"
      );
      if (beUnits[parsedIdx]) {
        return beUnits[parsedIdx].objective;
      }
    }
    const idx = isNaN(parsedIdx) ? 1 : parsedIdx + 1;
    return `Implement backend subtask ${idx} in an isolated sandbox environment.`;
  }
  if (step === WORKFLOW_STEPS.CODE_BACKEND) return "Implement backend business logic, db models, and controllers.";
  if (step.startsWith("code_frontend_")) {
    const parsedIdx = Number(step.substring("code_frontend_".length));
    if (!isNaN(parsedIdx) && analysisData?.execution_units) {
      const feUnits = analysisData.execution_units.filter(
        (unit: ExecutionUnit) => (unit.execution_profile?.agent?.toLowerCase() || "backend") === "frontend" || (unit.execution_profile?.agent?.toLowerCase() || "backend") === "fe"
      );
      if (feUnits[parsedIdx]) {
        return feUnits[parsedIdx].objective;
      }
    }
    const idx = isNaN(parsedIdx) ? 1 : parsedIdx + 1;
    return `Implement frontend UI subtask ${idx} in an isolated sandbox environment.`;
  }
  if (step === WORKFLOW_STEPS.CODE_FRONTEND) return "Create interactive web pages, layouts, and client-side logic.";
  if (step === WORKFLOW_STEPS.MERGE) return "Integrate backend and frontend branches together safely.";
  if (step === WORKFLOW_STEPS.REVIEW) return "Static analysis and automated check gates for quality control.";
  if (step === WORKFLOW_STEPS.FIX) return "Resolve any issues raised during the review step.";
  if (step === WORKFLOW_STEPS.TEST) return "Execute unit tests, integration tests, and check exit status.";
  if (step === WORKFLOW_STEPS.PR) return "Generate pull request description and submit changes.";
  return "Workflow execution step.";
}

export function getStepTasks(step: string, analysisData?: Partial<TaskAnalysis>): string[] {
  if (step.startsWith("code_backend_")) {
    const parsedIdx = Number(step.substring("code_backend_".length));
    if (!isNaN(parsedIdx) && analysisData?.execution_units) {
      const beUnits = analysisData.execution_units.filter(
        (unit: ExecutionUnit) => (unit.execution_profile?.agent?.toLowerCase() || "backend") !== "frontend" && (unit.execution_profile?.agent?.toLowerCase() || "backend") !== "fe"
      );
      if (beUnits[parsedIdx]?.tasks) {
        return beUnits[parsedIdx].tasks;
      }
    }
  }
  if (step.startsWith("code_frontend_")) {
    const parsedIdx = Number(step.substring("code_frontend_".length));
    if (!isNaN(parsedIdx) && analysisData?.execution_units) {
      const feUnits = analysisData.execution_units.filter(
        (unit: ExecutionUnit) => (unit.execution_profile?.agent?.toLowerCase() || "backend") === "frontend" || (unit.execution_profile?.agent?.toLowerCase() || "backend") === "fe"
      );
      if (feUnits[parsedIdx]?.tasks) {
        return feUnits[parsedIdx].tasks;
      }
    }
  }
  return [];
}

export function getSemanticStatusColor(status: string): { bg: string; text: string; border: string; ring: string; dot: string } {
  const norm = status?.toLowerCase() ?? "";
  if (norm === "success" || norm === "recorded" || norm === "skipped" || norm === "completed" || norm === "merged") {
    return {
      bg: "bg-emerald-500/10",
      text: "text-emerald-500",
      border: "border-emerald-500",
      ring: "ring-emerald-500/30",
      dot: "bg-emerald-500"
    };
  }
  if (norm === "running" || norm === "active") {
    return {
      bg: "bg-sky-500/10",
      text: "text-sky-500",
      border: "border-sky-500",
      ring: "ring-sky-500/30",
      dot: "bg-sky-500"
    };
  }
  if (norm === "failed" || norm === "blocked" || norm === "danger") {
    return {
      bg: "bg-rose-500/10",
      text: "text-rose-500",
      border: "border-rose-500",
      ring: "ring-rose-500/30",
      dot: "bg-rose-500"
    };
  }
  return {
    bg: "bg-slate-500/10",
    text: "text-content-muted",
    border: "border-stroke",
    ring: "ring-transparent",
    dot: "bg-slate-400"
  };
}

// getTaskSemanticStatus buckets a Task.status value (todo/context_loading/analyzing/
// spec_review/planning/coding/reviewing/fixing/testing/pr_ready/human_review/merged/failed —
// see badge.tsx's taskStatusBadge) into the 4-state vocabulary getSemanticStatusColor expects
// (success/running/failed/pending). Task statuses use a different, richer vocabulary than the
// workflow-step statuses (success/running/failed/pending) getSemanticStatusColor was written
// for, so passing task.status to it directly only ever matches "merged" or "failed" — every
// other (in-progress) status silently fell through to the gray "pending" default.
export function getTaskSemanticStatus(status: string): "success" | "running" | "failed" | "pending" {
  const norm = status?.toLowerCase() ?? "";
  if (norm === "merged") return "success";
  if (norm === "failed") return "failed";
  // todo hasn't started yet; spec_review/human_review are paused waiting on a human, not
  // actively worked on by the agent — both read as "waiting", not "running".
  if (norm === "todo" || norm === "spec_review" || norm === "human_review") return "pending";
  // context_loading, analyzing, planning, coding, reviewing, fixing, testing, pr_ready
  return "running";
}

interface TaskDetailContextType {
  projectID: string;
  taskID: string;
  token: string;
  task: Task | undefined;
  workflow: WorkflowStatus | undefined;
  logs: RealtimeLog[];
  submittingPR: boolean;
  error: string | null;
  setError: (err: string) => void;
  feedback: string;
  setFeedback: (feedback: string) => void;
  isRequestingChanges: boolean;
  setIsRequestingChanges: (val: boolean) => void;
  specFeedbackText: string;
  setSpecFeedbackText: (val: string) => void;
  execute: () => Promise<void>;
  analyze: () => Promise<void>;
  retry: () => Promise<void>;
  pause: () => Promise<void>;
  cancel: () => Promise<void>;
  approveSpec: () => Promise<void>;
  requestSpecChanges: () => void;
  submitSpecChanges: () => Promise<void>;
  approvePR: () => Promise<void>;
  rejectPR: () => Promise<void>;
  startReview: () => Promise<void>;
  deleteTask: () => Promise<boolean>;
  updateTask: (fields: Partial<Task>) => Promise<boolean>;
  mutateWorkflow: () => Promise<unknown>;
  isTaskLoading: boolean;
  workflowError: Error | null;

  activeSpecTab: "summary" | "proposal" | "specs" | "design" | "tasks";
  setActiveSpecTab: (tab: "summary" | "proposal" | "specs" | "design" | "tasks") => void;
  completedPlanSteps: Record<number, boolean>;
  togglePlanStep: (idx: number) => void;

  // Computed Values
  analysisData: Partial<TaskAnalysis>;
  workflowSteps: string[];
  stepMetadata: Map<string, { status: string; timestamp?: string; error?: string }>;
  latest: Map<string, string>;
  stepDurations: Map<string, string>;
  workflowCompletion: number;
  workflowStatusCounts: { done: number; running: number; failed: number; pending: number };
  clarificationQuestions: string[];
  prSummaries: Array<{ title?: string; body?: string; review_limit_exceeded?: boolean }>;
  displayFiles: string[];
  riskAssessment: { level: string; reason: string };
  descriptionParts: { body: string; context: string };
  isReviewWaiting: boolean;
  isPRMerged: boolean;
  hasPR: boolean;
  isExecutionReady: boolean;
  isPaused: boolean;
  diffText: string;
  parsedDiffs: ParsedFileDiff[];
  parsedDiffFiles: string[];
  implementationItems: ImplementationItem[];
  currentImplementationItem: ImplementationItem | null;
}

const TaskDetailContext = createContext<TaskDetailContextType | undefined>(undefined);

export function TaskDetailProvider({
  projectID,
  taskID,
  children,
}: {
  projectID: string;
  taskID: string;
  children: React.ReactNode;
}) {
  const session = useSession();
  const token = session?.token ?? "";

  const [activeSpecTab, setActiveSpecTab] = useState<"summary" | "proposal" | "specs" | "design" | "tasks">("summary");
  const [completedPlanSteps, setCompletedPlanSteps] = useState<Record<number, boolean>>({});

  const togglePlanStep = useCallback((idx: number) => {
    setCompletedPlanSteps((prev) => ({ ...prev, [idx]: !prev[idx] }));
  }, []);

  const {
    task,
    workflow,
    logs,
    error,
    setError,
    submittingPR,
    feedback,
    setFeedback,
    isRequestingChanges,
    setIsRequestingChanges,
    specFeedbackText,
    setSpecFeedbackText,
    execute,
    analyze,
    retry,
    pause,
    cancel,
    approveSpec,
    requestSpecChanges,
    submitSpecChanges,
    approvePR,
    rejectPR,
    startReview,
    deleteTask,
    updateTask,
    mutateWorkflow,
    isLoading: isTaskLoading,
    workflowError,
  } = useTaskWorkflow(taskID);

  // Fetch live workspace artifacts for the active workflow job
  const jobID = workflow?.job?.id;
  const { data: artifacts } = useAuthedSWR(
    jobID ? ["workflow-artifacts", jobID] : null,
    (token) => api.taskArtifacts(jobID!, token),
  );

  // Parse task analysis
  const analysisData = useMemo(() => {
    let data: Partial<TaskAnalysis> = {};
    try {
      if (task?.analysis) {
        data = typeof task.analysis === "string" ? JSON.parse(task.analysis) : task.analysis;
      }
    } catch { }

    if (task && task.spec_status && task.spec_status !== "none") {
      if (!data.proposal_md) {
        data.proposal_md = `## Proposal for ${task.title}\n\n${task.description || ""}\n`;
      }
      if (!data.specs_md) {
        data.specs_md = `## ADDED Requirements\n\n### Requirement: ${task.title}\n${task.description || ""}\n`;
      }
      if (!data.design_md) {
        data.design_md = `## Design\n\nImplementation design details.\n`;
      }
      if (!data.tasks_md) {
        let tasksContent = "## Tasks\n\n";
        if (data.execution_plan && data.execution_plan.length > 0) {
          tasksContent += data.execution_plan.map(step => `- [ ] ${step}`).join("\n") + "\n";
        } else {
          tasksContent += "- [ ] Implement changes\n";
        }
        data.tasks_md = tasksContent;
      }
    }

    return data;
  }, [task]);

  const workflowSteps = useMemo(() => {
    if (task?.complexity === "easy") {
      return EASY_STEPS;
    }
    if (task?.complexity === "medium" || task?.complexity === "hard") {
      const steps: string[] = [WORKFLOW_STEPS.CONTEXT_LOAD, WORKFLOW_STEPS.ANALYZE, WORKFLOW_STEPS.PLAN];

      let backendCount = 0;
      let frontendCount = 0;

      if (analysisData.execution_units && analysisData.execution_units.length > 0) {
        analysisData.execution_units.forEach((unit) => {
          const agent = unit.execution_profile?.agent?.toLowerCase() || "backend";
          if (agent === "frontend" || agent === "fe") {
            frontendCount++;
          } else {
            backendCount++;
          }
        });
      } else {
        const subtasks = parseTasksMD(analysisData.tasks_md);
        backendCount = subtasks.backend.length;
        frontendCount = subtasks.frontend.length;
      }

      const hasExecutionUnits = !!(analysisData.execution_units && analysisData.execution_units.length > 0);

      if (hasExecutionUnits) {
        if (backendCount > 0) {
          for (let i = 0; i < backendCount; i++) {
            steps.push(`code_backend_${i}`);
          }
        }
        if (frontendCount > 0) {
          for (let i = 0; i < frontendCount; i++) {
            steps.push(`code_frontend_${i}`);
          }
        }
        if (backendCount === 0 && frontendCount === 0) {
          steps.push(WORKFLOW_STEPS.CODE_BACKEND, WORKFLOW_STEPS.CODE_FRONTEND);
        }
      } else {
        if (backendCount > 0) {
          for (let i = 0; i < backendCount; i++) {
            steps.push(`code_backend_${i}`);
          }
        } else {
          steps.push(WORKFLOW_STEPS.CODE_BACKEND);
        }

        if (frontendCount > 0) {
          for (let i = 0; i < frontendCount; i++) {
            steps.push(`code_frontend_${i}`);
          }
        } else {
          steps.push(WORKFLOW_STEPS.CODE_FRONTEND);
        }
      }


      steps.push(
        WORKFLOW_STEPS.MERGE,
        WORKFLOW_STEPS.REVIEW,
        WORKFLOW_STEPS.FIX,
        WORKFLOW_STEPS.TEST,
        WORKFLOW_STEPS.PR
      );
      return steps;
    }
    return [WORKFLOW_STEPS.CONTEXT_LOAD, WORKFLOW_STEPS.ANALYZE];
  }, [task?.complexity, analysisData.tasks_md, analysisData.execution_units]);

  const stepMetadata = useMemo(() => {
    const map = new Map<string, { status: string; timestamp?: string; error?: string }>();
    for (const checkpoint of workflow?.checkpoints ?? []) {
      const status = checkpoint.state?.status;
      const error = checkpoint.state?.error;
      map.set(checkpoint.step, {
        status: typeof status === "string" ? status : "recorded",
        timestamp: checkpoint.created_at,
        error: typeof error === "string" ? error : undefined,
      });
    }
    return map;
  }, [workflow]);

  const latest = useMemo(() => {
    const map = new Map<string, string>();
    for (const [step, info] of stepMetadata.entries()) {
      map.set(step, info.status);
    }
    return map;
  }, [stepMetadata]);

  const stepDurations = useMemo(() => {
    const map = new Map<string, string>();
    if (!workflow?.checkpoints || workflow.checkpoints.length === 0) return map;

    const checkpointsByStep = new Map<string, typeof workflow.checkpoints>();
    for (const cp of workflow.checkpoints) {
      if (!workflowSteps.includes(cp.step)) continue;
      const list = checkpointsByStep.get(cp.step) || [];
      list.push(cp);
      checkpointsByStep.set(cp.step, list);
    }

    let previousStepEnd: number | null = null;
    for (const step of workflowSteps) {
      const cps = checkpointsByStep.get(step);
      if (!cps || cps.length === 0) continue;

      const sortedCps = [...cps].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
      const firstCp = sortedCps[0];
      const lastCp = sortedCps[sortedCps.length - 1];

      let startMs = new Date(firstCp.created_at).getTime();
      if (sortedCps.length === 1 && firstCp.state?.status !== "running" && previousStepEnd !== null) {
        startMs = previousStepEnd;
      }

      const isRunning = lastCp.state?.status === "running";
      const endMs = isRunning ? Date.now() : new Date(lastCp.created_at).getTime();
      previousStepEnd = endMs;

      const durationSec = Math.max(0, Math.round((endMs - startMs) / 1000));
      if (durationSec < 60) {
        map.set(step, `${durationSec}s`);
      } else {
        const min = Math.floor(durationSec / 60);
        const sec = durationSec % 60;
        map.set(step, `${min}m ${sec}s`);
      }
    }
    return map;
  }, [workflow, workflowSteps]);

  // Inexpensive calculations can be direct (removed excess memo blocks or kept simple)
  const workflowCompletion = useMemo(() => {
    const finished = workflowSteps.filter((step) => {
      const status = latest.get(step);
      return status === "success" || status === "recorded" || status === "skipped";
    }).length;
    return Math.round((finished / workflowSteps.length) * 100);
  }, [latest, workflowSteps]);

  const workflowStatusCounts = useMemo(() => {
    const counts = { done: 0, running: 0, failed: 0, pending: 0 };
    for (const step of workflowSteps) {
      const status = latest.get(step) ?? "pending";
      if (status === "success" || status === "recorded" || status === "skipped") {
        counts.done += 1;
      } else if (status === "running") {
        counts.running += 1;
      } else if (status === "failed") {
        counts.failed += 1;
      } else {
        counts.pending += 1;
      }
    }
    return counts;
  }, [latest, workflowSteps]);

  // Find the latest diff/patch artifact from the runner run (sorted by created_at)
  const latestDiffArtifact = useMemo(() => {
    if (!artifacts) return null;
    const diffArts = artifacts.filter((art: WorkflowArtifact) => art.type === "diff" || art.type === "patch");
    if (diffArts.length === 0) return null;
    // Sort by created_at safely to guarantee order
    return [...diffArts].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())[diffArts.length - 1];
  }, [artifacts]);

  const diffText = useMemo(() => {
    if (!latestDiffArtifact) return "";
    const payload = latestDiffArtifact.payload;
    if (typeof payload === "string") return payload;
    if (payload && typeof payload === "object") {
      const obj = payload as Record<string, unknown>;
      return (typeof obj.diff === "string" ? obj.diff : "") ||
        (typeof obj.patch === "string" ? obj.patch : "") ||
        JSON.stringify(payload);
    }
    return "";
  }, [latestDiffArtifact]);

  const parsedDiffs = useMemo(() => {
    return parseUnifiedDiff(diffText);
  }, [diffText]);

  const parsedDiffFiles = useMemo(() => {
    return parsedDiffs.map((d) => d.filename);
  }, [parsedDiffs]);

  const clarificationQuestions = useMemo(() => {
    return Array.isArray(analysisData.clarification_questions)
      ? (analysisData.clarification_questions as unknown[]).filter(
        (question): question is string => typeof question === "string" && question.trim().length > 0,
      )
      : [];
  }, [analysisData.clarification_questions]);

  const prSummaries = useMemo(() => {
    if (!task?.pr_metadata) return [];
    try {
      const metadata = typeof task.pr_metadata === "string" ? JSON.parse(task.pr_metadata) : task.pr_metadata;
      if (Array.isArray(metadata)) {
        return metadata;
      }
    } catch { }
    return [];
  }, [task?.pr_metadata]);

  const displayFiles = useMemo<string[]>(() => {
    if (prSummaries.length > 0 && prSummaries[0].changed_files) {
      return prSummaries[0].changed_files as string[];
    }
    const affectedFiles = analysisData.affected_files || [];
    const mapped = affectedFiles.map((f: string | AffectedFile) => isAffectedFile(f) ? f.file : typeof f === "string" ? f : "");
    return parsedDiffFiles.length > 0 ? parsedDiffFiles : mapped;
  }, [prSummaries, parsedDiffFiles, analysisData.affected_files]);

  const riskAssessment = useMemo(() => {
    if (prSummaries.length > 0 && prSummaries[0].risk_level) {
      return {
        level: prSummaries[0].risk_level,
        reason: prSummaries[0].risk_reason || "",
      };
    }
    return getRiskAssessment(task?.complexity ?? "easy", displayFiles, analysisData.risk_domains);
  }, [prSummaries, task?.complexity, displayFiles, analysisData.risk_domains]);

  const descriptionParts = useMemo(
    () => splitTaskDescription(task?.description ?? ""),
    [task?.description],
  );

  const isReviewWaiting = task?.status === "human_review";
  const isPRMerged = task?.status === "merged";
  const hasPR = !!(task?.pr_urls && task.pr_urls.length > 0);
  const isExecutionReady = !!(
    task &&
    (task.spec_status === "auto_approved" || task.spec_status === "approved") &&
    (task.status === "todo" || task.status === "failed")
  );
  const isPaused = workflow?.job?.status === "paused";

  const implementationItems = useMemo(() => {
    const currentStep = workflow?.job?.status === "running" ? workflow?.job?.step : undefined;
    return deriveImplementationItems(analysisData, workflow?.checkpoints, currentStep);
  }, [analysisData, workflow?.checkpoints, workflow?.job?.status, workflow?.job?.step]);

  const currentImplementationItem = useMemo(() => {
    return implementationItems.find(item => item.status === "running") || null;
  }, [implementationItems]);

  return (
    <TaskDetailContext.Provider
      value={{
        projectID,
        taskID,
        token,
        task,
        workflow,
        logs,
        error,
        setError,
        submittingPR,
        feedback,
        setFeedback,
        isRequestingChanges,
        setIsRequestingChanges,
        specFeedbackText,
        setSpecFeedbackText,
        execute,
        analyze,
        retry,
        pause,
        cancel,
        approveSpec,
        requestSpecChanges,
        submitSpecChanges,
        approvePR,
        rejectPR,
        startReview,
        deleteTask,
        updateTask,
        mutateWorkflow,
        isTaskLoading,
        workflowError,

        activeSpecTab,
        setActiveSpecTab,
        completedPlanSteps,
        togglePlanStep,

        // Computed
        analysisData,
        workflowSteps,
        stepMetadata,
        latest,
        stepDurations,
        workflowCompletion,
        workflowStatusCounts,
        clarificationQuestions,
        prSummaries,
        displayFiles,
        riskAssessment,
        descriptionParts,
        isReviewWaiting,
        isPRMerged,
        hasPR,
        isExecutionReady,
        isPaused,
        diffText,
        parsedDiffs,
        parsedDiffFiles,
        implementationItems,
        currentImplementationItem,
      }}
    >
      {children}
    </TaskDetailContext.Provider>
  );
}

export function useTaskDetail() {
  const context = useContext(TaskDetailContext);
  if (!context) {
    throw new Error("useTaskDetail must be used within a TaskDetailProvider");
  }
  return context;
}
