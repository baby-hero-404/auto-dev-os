import { useState } from "react";
import { Edit3, Trash2, Loader2, Save, X, Check, ShieldAlert, Info } from "lucide-react";
import { toast } from "sonner";
import { api, ApiError } from "@/lib/api";
import type { Rule } from "@/lib/types";
import { RuleEnforcementBadge, EnforcementToggle } from "./RuleUI";

export function RuleListItem({
  rule,
  token,
  isNew,
  onUpdate,
  onDelete,
}: {
  rule: Rule;
  token: string;
  isNew: boolean;
  onUpdate: (rule: Rule) => void;
  onDelete: (id: string) => void;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [editRuleContent, setEditRuleContent] = useState(rule.content);
  const [editRuleEnforcement, setEditRuleEnforcement] = useState<Rule["enforcement"]>(rule.enforcement);
  const [isSaving, setIsSaving] = useState(false);
  const [savedSuccess, setSavedSuccess] = useState(false);

  const [isConfirmingDelete, setIsConfirmingDelete] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  const isStrict = rule.enforcement === "strict";

  function startEdit() {
    setIsEditing(true);
    setEditRuleContent(rule.content);
    setEditRuleEnforcement(rule.enforcement);
    setIsConfirmingDelete(false);
  }

  function cancelEdit() {
    setIsEditing(false);
  }

  async function handleUpdate() {
    if (!token || isSaving) return;
    const content = editRuleContent.trim();
    if (!content) {
      toast.error("Rule content cannot be empty.");
      return;
    }

    setIsSaving(true);
    try {
      const updatedRule = await api.updateRule(rule.id, token, {
        content,
        enforcement: editRuleEnforcement,
      });
      onUpdate(updatedRule);
      setIsEditing(false);
      setSavedSuccess(true);
      toast.success("Rule updated successfully!");
      setTimeout(() => setSavedSuccess(false), 2000);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to update rule");
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDelete() {
    if (!token || isDeleting) return;

    setIsDeleting(true);
    try {
      await api.deleteRule(rule.id, token);
      onDelete(rule.id);
      toast.success("Rule deleted successfully!");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete rule");
      setIsDeleting(false);
    }
  }

  return (
    <div
      className={`group rounded-lg border border-stroke bg-panel/30 p-4.5 text-sm transition-all duration-200 hover:bg-panel/60 hover:shadow-xs ${
        isStrict
          ? "border-l-4 border-l-rose-500/80"
          : "border-l-4 border-l-amber-500/80"
      } ${
        isNew ? "ring-2 ring-brand-primary bg-brand-primary/5 border-brand-primary" : ""
      } ${isDeleting ? "opacity-40" : "opacity-100"}`}
    >
      {isEditing ? (
        <div className="flex gap-4">
          <div className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border ${
            isStrict
              ? "border-rose-500/20 bg-rose-500/5 text-rose-500"
              : "border-amber-500/20 bg-amber-500/5 text-amber-500"
          }`}>
            {isStrict ? <ShieldAlert size={16} /> : <Info size={16} />}
          </div>
          <div className="flex-1 space-y-3">
            <textarea
              value={editRuleContent}
              onChange={(e) => setEditRuleContent(e.target.value)}
              className="min-h-[80px] w-full resize-none rounded-lg border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none focus:ring-1 focus:ring-brand-primary/20"
              disabled={isSaving}
            />
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <EnforcementToggle
                value={editRuleEnforcement}
                onChange={setEditRuleEnforcement}
                disabled={isSaving}
              />
              <div className="flex justify-end gap-2">
                <button
                  onClick={cancelEdit}
                  disabled={isSaving}
                  className="inline-flex items-center gap-1.5 rounded-lg border border-stroke px-3 py-1.5 text-xs font-semibold text-foreground transition hover:bg-surface disabled:opacity-50 cursor-pointer"
                  type="button"
                >
                  <X size={13} />
                  Cancel
                </button>
                <button
                  onClick={handleUpdate}
                  disabled={isSaving || !editRuleContent.trim()}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-brand-primary px-3 py-1.5 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                  type="button"
                >
                  {isSaving ? <Loader2 size={13} className="animate-spin" /> : <Save size={13} />}
                  Save Changes
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="flex items-start justify-between gap-4">
            <div className="flex gap-4 min-w-0">
              <div className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border ${
                isStrict
                  ? "border-rose-500/20 bg-rose-500/5 text-rose-500"
                  : "border-amber-500/20 bg-amber-500/5 text-amber-500"
              }`}>
                {isStrict ? <ShieldAlert size={16} /> : <Info size={16} />}
              </div>

              <div className="min-w-0">
                <p className="whitespace-pre-wrap text-foreground/90 font-medium leading-relaxed">{rule.content}</p>
                <div className="mt-3 flex flex-wrap items-center gap-2">
                  <RuleEnforcementBadge enforcement={rule.enforcement} />
                  <span className="rounded-full border border-stroke bg-surface px-2.5 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted shadow-xs">
                    {rule.scope}
                  </span>
                  {savedSuccess && (
                    <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                      <Check size={13} className="stroke-[3]" />
                      Saved
                    </span>
                  )}
                </div>
              </div>
            </div>

            <div className="flex shrink-0 gap-1 opacity-80 group-hover:opacity-100 transition-opacity duration-150">
              <button
                onClick={startEdit}
                className="rounded-lg p-2 text-content-muted transition hover:bg-surface hover:text-foreground cursor-pointer"
                title="Edit rule"
                type="button"
              >
                <Edit3 size={14} />
              </button>
              <button
                onClick={() => setIsConfirmingDelete(true)}
                className="rounded-lg p-2 text-content-muted transition hover:bg-rose-500/10 hover:text-rose-600 dark:hover:text-rose-400 cursor-pointer"
                title="Delete rule"
                type="button"
              >
                <Trash2 size={14} />
              </button>
            </div>
          </div>
          {isConfirmingDelete && (
            <div className="mt-3 flex flex-col gap-2 rounded-lg border border-red-500/20 bg-red-500/5 p-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-xs font-semibold text-red-700 dark:text-red-300">Are you sure you want to delete this rule?</p>
              <div className="flex gap-2">
                <button
                  onClick={handleDelete}
                  disabled={isDeleting}
                  className="inline-flex items-center gap-1 rounded bg-red-500 px-3 py-1 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                  type="button"
                >
                  {isDeleting && <Loader2 size={12} className="animate-spin" />}
                  Delete
                </button>
                <button
                  onClick={() => setIsConfirmingDelete(false)}
                  disabled={isDeleting}
                  className="rounded border border-stroke bg-surface px-3 py-1 text-xs font-semibold text-foreground transition hover:bg-panel-muted disabled:opacity-50 cursor-pointer"
                  type="button"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
