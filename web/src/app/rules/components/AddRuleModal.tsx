import { useState, FormEvent } from "react";
import { ShieldCheck, Plus, Loader2, X } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import type { Rule } from "@/lib/types";
import { EnforcementToggle } from "./RuleUI";

export function AddRuleModal({
  orgID,
  token,
  isOpen,
  onClose,
  onSuccess,
}: {
  orgID: string;
  token: string;
  isOpen: boolean;
  onClose: () => void;
  onSuccess: (rule: Rule) => void;
}) {
  const [ruleContent, setRuleContent] = useState("");
  const [ruleEnforcement, setRuleEnforcement] = useState<Rule["enforcement"]>("strict");
  const [isAddingRule, setIsAddingRule] = useState(false);
  const [ruleError, setRuleError] = useState("");

  if (!isOpen) return null;

  async function handleAddRule(e: FormEvent) {
    e.preventDefault();
    if (!orgID || !token) return;

    const content = ruleContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }

    setRuleError("");
    setIsAddingRule(true);
    try {
      const createdRule = await api.createGlobalRule(orgID, token, {
        content,
        enforcement: ruleEnforcement,
      });
      toast.success("Global rule added successfully!");
      setRuleContent("");
      setRuleEnforcement("strict");
      onSuccess(createdRule);
      onClose();
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to add rule");
    } finally {
      setIsAddingRule(false);
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4 backdrop-blur-xs">
      <div className="relative w-full max-w-lg rounded-xl border border-stroke bg-card p-6 shadow-2xl animate-in fade-in zoom-in duration-200">
        <button
          onClick={onClose}
          disabled={isAddingRule}
          className="absolute right-4 top-4 rounded-lg p-2 text-content-muted transition hover:bg-surface hover:text-foreground disabled:opacity-50 cursor-pointer"
          type="button"
        >
          <X size={16} />
        </button>

        <h3 className="mb-5 flex items-center gap-2.5 font-mono text-lg font-bold text-foreground">
          <ShieldCheck size={20} className="text-brand-primary" />
          Add Global Rule
        </h3>

        <form onSubmit={handleAddRule} className="space-y-5">
          <div>
            <label className="mb-1.5 block font-mono text-xs font-bold uppercase tracking-wider text-content-muted">
              Rule content *
            </label>
            <textarea
              value={ruleContent}
              onChange={(e) => setRuleContent(e.target.value)}
              placeholder="Always write unit tests for every new function."
              className="min-h-[120px] w-full resize-none rounded-lg border border-stroke bg-background px-3 py-2.5 text-sm text-foreground focus:border-brand-primary focus:outline-none focus:ring-1 focus:ring-brand-primary/20 placeholder:text-content-muted/65"
              disabled={isAddingRule}
              required
            />
          </div>

          <div>
            <label className="mb-1.5 block font-mono text-xs font-bold uppercase tracking-wider text-content-muted">
              Enforcement *
            </label>
            <EnforcementToggle
              value={ruleEnforcement}
              onChange={setRuleEnforcement}
              disabled={isAddingRule}
              showDescriptions
            />
          </div>

          {ruleError && <p className="rounded border border-red-500/20 bg-red-500/10 p-2 text-xs font-medium text-red-600 dark:text-red-300">{ruleError}</p>}

          <div className="flex justify-end gap-2 pt-2">
            <button
              onClick={onClose}
              disabled={isAddingRule}
              className="rounded-lg border border-stroke px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface disabled:opacity-50 cursor-pointer"
              type="button"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isAddingRule || !ruleContent.trim()}
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer shadow-sm"
            >
              {isAddingRule ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
              Add Rule
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
