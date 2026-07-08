"use client";

import { useState, FormEvent } from "react";
import { Loader2, Plus } from "lucide-react";
import type { Rule } from "@/lib/types";
import { EnforcementToggle } from "./EnforcementToggle";
import { ApiError } from "@/lib/api";

interface AddRuleFormProps {
  onAddRule: (content: string, enforcement: Rule["enforcement"]) => Promise<Rule>;
  setNewRuleID: (id: string) => void;
  ruleError: string;
  setRuleError: (err: string) => void;
}

export function AddRuleForm({ onAddRule, setNewRuleID, ruleError, setRuleError }: AddRuleFormProps) {
  const [ruleContent, setRuleContent] = useState("");
  const [ruleEnforcement, setRuleEnforcement] = useState<Rule["enforcement"]>("strict");
  const [isAddingRule, setIsAddingRule] = useState(false);

  async function handleAddRule(e: FormEvent) {
    e.preventDefault();
    const content = ruleContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }
    setRuleError("");
    setIsAddingRule(true);
    try {
      const created = await onAddRule(content, ruleEnforcement);
      setRuleContent("");
      setRuleEnforcement("strict");
      setNewRuleID(created.id);
      setTimeout(() => setNewRuleID(""), 200);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to add rule");
    } finally {
      setIsAddingRule(false);
    }
  }

  return (
    <div className="rounded-lg border border-stroke bg-card p-5 h-fit">
      <h3 className="font-sans font-semibold text-foreground mb-4">Add Project Rule</h3>
      <form onSubmit={handleAddRule} className="space-y-4">
        <div className="flex flex-col gap-1.5">
          <label className="block text-xs font-mono font-bold uppercase tracking-wider text-content-muted">
            Rule Guideline
          </label>
          <textarea
            id="project-rule-content"
            value={ruleContent}
            onChange={(e) => setRuleContent(e.target.value)}
            placeholder="e.g. Always write comprehensive unit tests using vitest for every new helper function."
            className="min-h-[100px] w-full rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none resize-none transition"
            disabled={isAddingRule}
            required
          />
        </div>

        <EnforcementToggle value={ruleEnforcement} onChange={setRuleEnforcement} disabled={isAddingRule} />

        {ruleError && (
          <div className="rounded border border-red-500/20 bg-red-500/10 p-2.5 text-xs text-red-200" role="alert">
            {ruleError}
          </div>
        )}

        <button
          type="submit"
          disabled={isAddingRule || !ruleContent.trim()}
          className="flex w-full items-center justify-center gap-2 rounded bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
        >
          {isAddingRule ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />} Add Rule
        </button>
      </form>
    </div>
  );
}
