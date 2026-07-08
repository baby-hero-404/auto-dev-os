"use client";

import { useState } from "react";
import { Check, Edit3, Loader2, Save, Trash2, X } from "lucide-react";
import type { Rule } from "@/lib/types";
import { RuleEnforcementBadge } from "./RuleEnforcementBadge";
import { EnforcementToggle } from "./EnforcementToggle";
import { ApiError } from "@/lib/api";

interface RuleCardProps {
  rule: Rule;
  isNew: boolean;
  onUpdateRule: (ruleId: string, content: string, enforcement: Rule["enforcement"]) => Promise<void>;
  onDeleteRule: (ruleId: string) => Promise<void>;
  setRuleError: (err: string) => void;
}

export function RuleCard({ rule, isNew, onUpdateRule, onDeleteRule, setRuleError }: RuleCardProps) {
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState(rule.content);
  const [editEnforcement, setEditEnforcement] = useState<Rule["enforcement"]>(rule.enforcement);
  const [isSaving, setIsSaving] = useState(false);
  const [isSaved, setIsSaved] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);

  async function handleUpdate() {
    const content = editContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }
    setRuleError("");
    setIsSaving(true);
    try {
      await onUpdateRule(rule.id, content, editEnforcement);
      setIsSaved(true);
      setIsEditing(false);
      setTimeout(() => setIsSaved(false), 2000);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to update rule");
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDelete() {
    setRuleError("");
    setIsDeleting(true);
    try {
      await onDeleteRule(rule.id);
      setConfirmDelete(false);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to delete rule");
    } finally {
      setIsDeleting(false);
    }
  }

  function startEdit() {
    setEditContent(rule.content);
    setEditEnforcement(rule.enforcement);
    setIsEditing(true);
    setRuleError("");
    setConfirmDelete(false);
  }

  return (
    <div
      className={`rounded-lg border border-stroke bg-card p-4 text-sm transition duration-200 ${
        isNew ? "animate-fade-in" : ""
      } ${isDeleting ? "opacity-40" : ""}`}
    >
      {isEditing ? (
        <div className="space-y-3">
          <textarea
            value={editContent}
            onChange={(e) => setEditContent(e.target.value)}
            className="min-h-[72px] w-full resize-none rounded-md border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none"
            disabled={isSaving}
          />
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <EnforcementToggle
              value={editEnforcement}
              onChange={setEditEnforcement}
              disabled={isSaving}
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setIsEditing(false)}
                disabled={isSaving}
                className="inline-flex items-center gap-1 rounded border border-stroke px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-surface cursor-pointer disabled:opacity-50"
                type="button"
              >
                <X size={13} /> Cancel
              </button>
              <button
                onClick={handleUpdate}
                disabled={isSaving || !editContent.trim()}
                className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1.5 text-xs font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer disabled:opacity-50"
                type="button"
              >
                {isSaving ? (
                  <Loader2 size={13} className="animate-spin" />
                ) : (
                  <Save size={13} />
                )}
                Save
              </button>
            </div>
          </div>
        </div>
      ) : (
        <>
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="whitespace-pre-wrap text-foreground">{rule.content}</p>
              <div className="mt-2.5 flex items-center gap-2">
                <RuleEnforcementBadge enforcement={rule.enforcement} />
                <span className="rounded border border-stroke bg-surface px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted">
                  {rule.scope}
                </span>
                {isSaved && (
                  <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-500">
                    <Check size={13} /> Saved
                  </span>
                )}
              </div>
            </div>
            <div className="flex shrink-0 gap-1.5">
              <button
                onClick={startEdit}
                className="rounded p-1 text-content-muted hover:bg-surface hover:text-foreground transition cursor-pointer"
                title="Edit rule"
                type="button"
              >
                <Edit3 size={14} />
              </button>
              <button
                onClick={() => setConfirmDelete(true)}
                className="rounded p-1 text-content-muted hover:bg-rose-500/10 hover:text-rose-500 transition cursor-pointer"
                title="Delete rule"
                type="button"
              >
                <Trash2 size={14} />
              </button>
            </div>
          </div>

          {confirmDelete && (
            <div className="mt-3 flex flex-col gap-2 rounded-md border border-rose-500/20 bg-rose-500/5 p-3 sm:flex-row sm:items-center sm:justify-between">
              <p className="text-xs font-semibold text-rose-800 dark:text-rose-200">
                Are you sure you want to delete this rule?
              </p>
              <div className="flex gap-2">
                <button
                  onClick={handleDelete}
                  disabled={isDeleting}
                  className="inline-flex items-center gap-1 rounded bg-rose-500 px-3 py-1 text-xs font-semibold text-white hover:opacity-90 cursor-pointer"
                  type="button"
                >
                  {isDeleting && <Loader2 size={12} className="animate-spin" />} Delete
                </button>
                <button
                  onClick={() => setConfirmDelete(false)}
                  disabled={isDeleting}
                  className="rounded border border-stroke px-3 py-1 text-xs font-semibold text-foreground hover:bg-surface cursor-pointer"
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
