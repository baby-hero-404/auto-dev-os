"use client";

import { GitBranch } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { GitAccountsTab } from "@/components/settings/git-accounts-tab";

export default function GitAccountsPage() {
  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-foreground">Git Accounts</h2>
          <p className="mt-1 text-sm text-content-muted">
            Connect organization-level Git credentials for clone, push, and pull request automation.
          </p>
        </div>
        <div className="hidden rounded-lg border border-stroke bg-card p-3 text-brand-primary shadow-sm sm:grid sm:size-12 sm:place-items-center">
          <GitBranch size={22} />
        </div>
      </div>

      <GitAccountsTab />
    </DashboardLayout>
  );
}
