"use client";

import { useState } from "react";
import { ShieldCheck } from "lucide-react";
import type { Rule } from "@/lib/types";
import { ApiError } from "@/lib/api";
import { RuleCard } from "./rules/RuleCard";
import { AddRuleForm } from "./rules/AddRuleForm";
import { RuleEnforcementBadge } from "./rules/RuleEnforcementBadge";
import { RulesSkeleton } from "./rules/RulesSkeleton";
import { Button } from "@/components/ui/button";

interface RulesViewProps {
  rules: Rule[];
  isRulesLoading: boolean;
  onAddRule: (content: string, enforcement: Rule["enforcement"]) => Promise<Rule>;
  onUpdateRule: (ruleId: string, content: string, enforcement: Rule["enforcement"]) => Promise<void>;
  onDeleteRule: (ruleId: string) => Promise<void>;
  onSeedRules: () => Promise<void>;
}

export function RulesView({
  rules,
  isRulesLoading,
  onAddRule,
  onUpdateRule,
  onDeleteRule,
  onSeedRules,
}: RulesViewProps) {
  const [ruleError, setRuleError] = useState("");
  const [newRuleID, setNewRuleID] = useState("");
  const [isSeedingRules, setIsSeedingRules] = useState(false);

  const globalRules = rules.filter((rule) => rule.scope === "global");
  const projectRules = rules.filter((rule) => rule.scope !== "global");

  async function handleSeedRules() {
    if (isSeedingRules) return;
    setRuleError("");
    setIsSeedingRules(true);
    try {
      await onSeedRules();
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to seed rules");
    } finally {
      setIsSeedingRules(false);
    }
  }

  return (
    <div className="grid gap-6 lg:grid-cols-[1fr_380px]">
      {/* Rules Manager Content */}
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <ShieldCheck size={18} className="text-brand-primary" />
            <h2 className="font-sans text-lg font-semibold text-foreground">Behavioral Rules</h2>
          </div>
          {projectRules.length === 0 && (
            <Button
              onClick={handleSeedRules}
              disabled={isSeedingRules}
              isLoading={isSeedingRules}
              variant="secondary"
              size="sm"
            >
              Auto-seed Default Rules
            </Button>
          )}
        </div>

        {isRulesLoading ? (
          <RulesSkeleton />
        ) : (
          <div className="space-y-4">
            {/* Global Rules Section */}
            {globalRules.length > 0 && (
              <div className="rounded-lg border border-brand-primary/20 bg-brand-primary/5 p-4">
                <div className="mb-3 flex items-center justify-between">
                  <h3 className="text-sm font-semibold text-foreground flex items-center gap-1.5">
                    <ShieldCheck size={16} className="text-brand-primary" />
                    Global Organization Rules
                  </h3>
                  <span className="rounded bg-brand-primary/10 px-2 py-0.5 text-[10px] font-bold text-brand-primary uppercase font-mono tracking-wide">
                    {globalRules.length} inherited
                  </span>
                </div>
                <div className="space-y-2.5">
                  {globalRules.map((rule) => (
                    <div key={rule.id} className="rounded-lg border border-stroke bg-card p-3.5 text-sm">
                      <p className="whitespace-pre-wrap text-foreground">{rule.content}</p>
                      <div className="mt-2.5 flex items-center gap-2">
                        <RuleEnforcementBadge enforcement={rule.enforcement} />
                        <span className="rounded border border-stroke bg-surface px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted">
                          global
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Project-Specific Rules */}
            <div className="space-y-2">
              {projectRules.length === 0 ? (
                <div className="rounded-lg border border-dashed border-stroke bg-card p-6 text-center">
                  <p className="font-sans text-sm font-semibold text-foreground">No project-specific rules yet.</p>
                  <p className="mt-1 text-xs text-content-muted">
                    Project rules supplement global guidelines and fine-tune AI behavior local to this workspace.
                  </p>
                </div>
              ) : (
                projectRules.map((rule) => (
                  <RuleCard
                    key={rule.id}
                    rule={rule}
                    isNew={newRuleID === rule.id}
                    onUpdateRule={onUpdateRule}
                    onDeleteRule={onDeleteRule}
                    setRuleError={setRuleError}
                  />
                ))
              )}
            </div>
          </div>
        )}
      </div>

      {/* Add Project Rule Form Card */}
      <AddRuleForm
        onAddRule={onAddRule}
        setNewRuleID={setNewRuleID}
        ruleError={ruleError}
        setRuleError={setRuleError}
      />
    </div>
  );
}
