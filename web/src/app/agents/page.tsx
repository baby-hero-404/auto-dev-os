"use client";

import { Bot } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { MembersPanel } from "@/components/settings/members-panel";

export default function AgentsPage() {
  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-foreground">Agents</h2>
          <p className="mt-1 text-sm text-content-muted">
            Manage organization members, default agent pool, and agent-specific tools.
          </p>
        </div>
        <div className="hidden rounded-lg border border-stroke bg-card p-3 text-brand-primary shadow-sm sm:grid sm:size-12 sm:place-items-center">
          <Bot size={22} />
        </div>
      </div>
      <MembersPanel />
    </DashboardLayout>
  );
}
