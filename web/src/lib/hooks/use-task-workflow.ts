import { useState, useEffect, useMemo } from "react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { useRealtimeLogStore, type RealtimeLog } from "@/lib/store/use-realtime-log-store";
import type { TaskLog } from "@/lib/types";

export function useTaskWorkflow(taskID: string) {
  const session = useSession();
  const token = session?.token ?? "";
  const [error, setError] = useState("");
  const [feedback, setFeedback] = useState("");
  const [submittingPR, setSubmittingPR] = useState(false);
  const [isRequestingChanges, setIsRequestingChanges] = useState(false);
  const [specFeedbackText, setSpecFeedbackText] = useState("");

  const realtimeLogs = useRealtimeLogStore((state) => state.logs);
  const appendLogs = useRealtimeLogStore((state) => state.appendLogs);
  const clearLogs = useRealtimeLogStore((state) => state.clearLogs);

  const { data: workflow, mutate: mutateWorkflow, error: workflowError } = useAuthedSWR(
    taskID ? ["workflow", taskID] : null,
    (token) => api.taskWorkflow(taskID, token),
    { refreshInterval: (latestWorkflow) => (isWorkflowTerminal(latestWorkflow?.job?.status) ? 0 : 2500) },
  );

  const { data: fetchedLogs, mutate: mutateLogs } = useAuthedSWR(
    taskID ? ["task-logs", taskID] : null,
    (token) => api.taskLogs(taskID, token),
    { refreshInterval: () => (isWorkflowTerminal(workflow?.job?.status) ? 0 : 3000) },
  );

  const logs = useMemo(
    () => realtimeLogs.filter((log) => log.streamId === taskID).slice(-200),
    [realtimeLogs, taskID],
  );

  useEffect(() => {
    clearLogs(taskID);
  }, [clearLogs, taskID]);

  useEffect(() => {
    if (!fetchedLogs) return;
    appendLogs(fetchedLogs.map((log) => toRealtimeLog(taskID, log)));
  }, [appendLogs, fetchedLogs, taskID]);

  const task = workflow?.task;

  async function execute() {
    if (!token) return;
    setError("");
    try {
      await api.executeTask(taskID, token);
      await Promise.all([mutateWorkflow(), mutateLogs()]);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to execute workflow");
    }
  }

  async function analyze() {
    if (!token) return;
    setError("");
    try {
      await api.analyzeTask(taskID, token);
      await Promise.all([mutateWorkflow(), mutateLogs()]);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to run analysis");
    }
  }

  async function retry() {
    if (!token) return;
    setError("");
    try {
      await api.retryTask(taskID, token);
      await Promise.all([mutateWorkflow(), mutateLogs()]);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to retry workflow");
    }
  }

  async function approveSpec() {
    if (!token) return;
    setError("");
    try {
      await api.approveTaskAnalysis(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to approve spec");
    }
  }

  async function submitSpecChanges() {
    if (!token || !specFeedbackText.trim()) return;
    setError("");
    setSubmittingPR(true);
    try {
      await api.requestTaskChanges(taskID, token, specFeedbackText.trim());
      setSpecFeedbackText("");
      setIsRequestingChanges(false);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to request changes");
    } finally {
      setSubmittingPR(false);
    }
  }

  async function approvePR() {
    if (!token) return;
    setError("");
    setSubmittingPR(true);
    try {
      await api.approvePR(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to approve PR");
    } finally {
      setSubmittingPR(false);
    }
  }

  async function rejectPR() {
    if (!token) return;
    if (!feedback.trim()) {
      setError("Feedback is required to reject the PR");
      return;
    }
    setError("");
    setSubmittingPR(true);
    try {
      await api.rejectPR(taskID, token, feedback.trim());
      setFeedback("");
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to reject PR");
    } finally {
      setSubmittingPR(false);
    }
  }

  async function startReview() {
    if (!token) return;
    setError("");
    setSubmittingPR(true);
    try {
      await api.startReview(taskID, token);
      await mutateWorkflow();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to start review");
    } finally {
      setSubmittingPR(false);
    }
  }

  function requestSpecChanges() {
    setIsRequestingChanges(true);
  }

  async function deleteTask() {
    if (!token) return false;
    setError("");
    try {
      await api.deleteTask(taskID, token);
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to delete task");
      return false;
    }
  }

  async function updateTask(fields: { title?: string; description?: string }) {
    if (!token) return false;
    setError("");
    try {
      await api.updateTask(taskID, token, fields);
      await mutateWorkflow();
      return true;
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to update task");
      return false;
    }
  }

  return {
    task,
    workflow,
    logs,
    error,
    setError,
    isLoading: !workflow && !workflowError && !!token,
    workflowError,
    feedback,
    setFeedback,
    submittingPR,
    isRequestingChanges,
    setIsRequestingChanges,
    specFeedbackText,
    setSpecFeedbackText,
    execute,
    analyze,
    retry,
    approveSpec,
    requestSpecChanges,
    submitSpecChanges,
    approvePR,
    rejectPR,
    startReview,
    deleteTask,
    updateTask,
    mutateWorkflow,
    mutateLogs,
  };
}

function isWorkflowTerminal(status?: string) {
  return status === "done" || status === "completed" || status === "failed" || status === "merged";
}

function toRealtimeLog(taskID: string, log: TaskLog): RealtimeLog {
  return {
    id: log.id,
    streamId: taskID,
    source: "workflow",
    level: log.level,
    message: log.message,
    createdAt: log.created_at,
    createdAtEpoch: Date.parse(log.created_at),
  };
}
