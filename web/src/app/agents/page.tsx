"use client";

import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { MembersPanel } from "@/components/settings/members-panel";

export default function AgentsPage() {
  return (
    <DashboardLayout>
      <div className="mb-6">
        <h2 className="font-mono text-2xl font-semibold">Agents Pool</h2>
        <p className="mt-1 text-sm text-content-muted">
          Legacy route for the organization agent pool. The same controls are available in Settings.
        </p>
      </div>
      <MembersPanel />
    </DashboardLayout>
  );
}
