import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { CreateTaskPayload } from "@/components/projects/create-task-panel";

export function useTaskActions(projectID: string, token: string, mutateTasks: () => void) {
  const [taskError, setTaskError] = useState("");
  const [isCreatingTask, setIsCreatingTask] = useState(false);
  const [isLoadingTask, setIsLoadingTask] = useState<Record<string, boolean>>({});

  async function createTask(payload: CreateTaskPayload) {
    if (!projectID || !token) return false;
    if (!payload.title) { setTaskError("Task title is required."); return false; }
    setTaskError("");
    setIsCreatingTask(true);
    try {
      await api.createTask(projectID, token, {
        title: payload.title, description: payload.description, complexity: payload.complexity,
        priority: payload.priority, labels: payload.labels, agent_id: payload.agent_id,
        repository_id: payload.repository_id,
      });
      mutateTasks();
      return true;
    } catch (err) {
      setTaskError(err instanceof ApiError ? err.message : "Failed to create task");
      return false;
    } finally { setIsCreatingTask(false); }
  }

  async function handleTaskAction(taskId: string, action: "analyze" | "execute" | "delete") {
    setIsLoadingTask((prev) => ({ ...prev, [taskId]: true }));
    setTaskError("");
    try {
      if (action === "analyze") await api.analyzeTask(taskId, token);
      if (action === "execute") await api.executeTask(taskId, token);
      if (action === "delete") await api.deleteTask(taskId, token);
      mutateTasks();
      return true;
    } catch (err) {
      console.error(err);
      setTaskError(err instanceof ApiError ? err.message : `Failed to ${action} task`);
      return false;
    } finally {
      setIsLoadingTask((prev) => ({ ...prev, [taskId]: false }));
    }
  }

  return { taskError, setTaskError, isCreatingTask, isLoadingTask, createTask, handleTaskAction };
}
