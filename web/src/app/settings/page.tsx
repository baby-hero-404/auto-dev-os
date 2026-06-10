"use client";

import { useState } from "react";
import { Bot, GitBranch, Settings } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { MembersPanel } from "@/components/settings/members-panel";
import { GitAccountsPanel } from "@/components/settings/git-accounts-panel";

type SettingsTab = "providers" | "members" | "git";

const tabs: Array<{ id: SettingsTab; label: string; icon: typeof Settings }> = [
  { id: "members", label: "Members", icon: Bot },
  { id: "git", label: "Git Accounts", icon: GitBranch },
];

export default function SettingsPage() {
  const [activeTab, setActiveTab] = useState<SettingsTab>("members");

  return (
    <DashboardLayout>
      <div className="mb-6">
        <h2 className="font-mono text-2xl font-semibold">Settings</h2>
        <p className="mt-1 text-sm text-content-muted">
          Organization-level defaults, agent pool, and planned integrations.
        </p>
      </div>

      <div className="mb-6 flex flex-wrap gap-2 border-b border-stroke">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`inline-flex items-center gap-2 border-b-2 px-1 pb-3 font-mono text-sm font-bold uppercase tracking-wider transition ${
                activeTab === tab.id
                  ? "border-brand-primary text-white"
                  : "border-transparent text-content-muted hover:text-white"
              }`}
              type="button"
            >
              <Icon size={16} />
              {tab.label}
            </button>
          );
        })}
      </div>



      {activeTab === "members" && <MembersPanel />}

      {activeTab === "git" && <GitAccountsPanel />}
    </DashboardLayout>
  );
}
