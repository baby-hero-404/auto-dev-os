"use client";

import { useState } from "react";
import useSWR from "swr";
import { ShieldCheck, Plus } from "lucide-react";
import { toast, Toaster } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import type { Rule } from "@/lib/types";
import { DEFAULT_GLOBAL_RULES } from "./utils";
import { AddRuleModal } from "./components/AddRuleModal";
import { RuleList } from "./components/RuleList";

export default function RulesPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: rules = [], mutate: mutateRules, isLoading: isRulesLoading } = useSWR(
    orgID && token ? ["global-rules", orgID] : null,
    () => api.listGlobalRules(orgID, token),
  );

  const [isSeedingRules, setIsSeedingRules] = useState(false);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);
  const [newRuleID, setNewRuleID] = useState("");
  const [searchQuery, setSearchQuery] = useState("");

  async function handleSeedRules() {
    if (!orgID || !token || isSeedingRules) return;
    setIsSeedingRules(true);
    try {
      const seededRules = await api.seedGlobalRules(orgID, token);
      toast.success("Default global rules populated!");
      mutateRules([...seededRules, ...rules], false);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        try {
          const createdRules = await Promise.all(
            DEFAULT_GLOBAL_RULES.map((rule) => api.createGlobalRule(orgID, token, rule)),
          );
          toast.success("Default global rules populated!");
          mutateRules([...createdRules, ...rules], false);
        } catch (fallbackErr) {
          toast.error(fallbackErr instanceof ApiError ? fallbackErr.message : "Failed to seed global rules");
        }
      } else {
        toast.error(err instanceof ApiError ? err.message : "Failed to seed global rules");
      }
    } finally {
      setIsSeedingRules(false);
    }
  }

  function handleRuleAdded(createdRule: Rule) {
    setNewRuleID(createdRule.id);
    mutateRules([createdRule, ...rules], false);
    window.setTimeout(() => {
      setNewRuleID((current) => (current === createdRule.id ? "" : current));
    }, 2000);
  }

  function handleUpdateRule(updatedRule: Rule) {
    mutateRules(
      rules.map((item) => (item.id === updatedRule.id ? updatedRule : item)),
      false
    );
  }

  function handleDeleteRule(ruleID: string) {
    mutateRules(
      rules.filter((rule) => rule.id !== ruleID),
      false
    );
  }

  return (
    <DashboardLayout>
      <Toaster closeButton position="top-right" richColors />
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-2xl font-bold text-foreground tracking-tight flex items-center gap-2">
            <ShieldCheck size={26} className="text-brand-primary" />
            Global Rules
          </h2>
          <p className="mt-1 text-sm text-content-muted">
            Organization-wide constraints injected into every agent system prompt.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => setIsAddModalOpen(true)}
            className="inline-flex items-center gap-2 rounded-lg bg-brand-primary px-4 py-2.5 text-sm font-semibold text-slate-950 shadow-sm transition hover:opacity-90 active:scale-98 cursor-pointer"
            type="button"
          >
            <Plus size={16} className="stroke-[2.5]" />
            Add Global Rule
          </button>
        </div>
      </div>

      <div className="space-y-6">
        <div className="grid gap-6">
          <RuleList
            rules={rules}
            isLoading={isRulesLoading}
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            onSeedRules={handleSeedRules}
            isSeeding={isSeedingRules}
            onOpenAddModal={() => setIsAddModalOpen(true)}
            newRuleID={newRuleID}
            onUpdateRule={handleUpdateRule}
            onDeleteRule={handleDeleteRule}
            token={token}
          />
        </div>
      </div>

      <AddRuleModal
        orgID={orgID}
        token={token}
        isOpen={isAddModalOpen}
        onClose={() => setIsAddModalOpen(false)}
        onSuccess={handleRuleAdded}
      />
    </DashboardLayout>
  );
}
