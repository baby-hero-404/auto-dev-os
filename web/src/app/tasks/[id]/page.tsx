"use client";

import { use, useEffect } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { useSession, useIsHydrated } from "@/lib/session";
import useSWR from "swr";

export default function TaskRedirectPage({ params }: { params: Promise<{ id: string }> }) {
  const { id: taskID } = use(params);
  const session = useSession();
  const isHydrated = useIsHydrated();
  const router = useRouter();

  const token = session?.token ?? "";

  const { data: workflow } = useSWR(
    taskID && token ? ["workflow", taskID] : null,
    () => api.taskWorkflow(taskID, token)
  );

  useEffect(() => {
    if (!session) {
      router.replace("/");
      return;
    }
    if (workflow?.task?.project_id) {
      router.replace(`/projects/${workflow.task.project_id}/tasks/${taskID}`);
    }
  }, [workflow, taskID, router, session]);

  if (!isHydrated) return null;

  if (!session) {
    return (
      <div className="grid min-h-screen place-items-center bg-background font-mono text-sm text-content-muted">
        Redirecting to login...
      </div>
    );
  }

  return (
    <div className="grid min-h-screen place-items-center bg-background font-mono text-sm text-content-muted">
      <div className="flex flex-col items-center gap-3">
        <span className="size-6 animate-spin rounded-full border-2 border-brand-primary border-t-transparent" />
        <span>Redirecting to project task dashboard...</span>
      </div>
    </div>
  );
}
