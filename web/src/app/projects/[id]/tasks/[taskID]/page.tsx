"use client";

import { use } from "react";
import Link from "next/link";
import { TaskDetailProvider } from "./components/TaskDetailContext";
import { TaskDetailLayout } from "./components/TaskDetailLayout";
import { useSession } from "@/lib/session";

export default function ProjectTaskDetailPage({
  params,
}: {
  params: Promise<{ id: string; taskID: string }>;
}) {
  const { id: projectID, taskID } = use(params);
  const session = useSession();

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-card p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening a task.</p>
          <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">Back to login</Link>
        </div>
      </main>
    );
  }

  return (
    <TaskDetailProvider projectID={projectID} taskID={taskID}>
      <TaskDetailLayout />
    </TaskDetailProvider>
  );
}
