"use client";

import { useState } from "react";
import { Check, Edit3, Trash2, X } from "lucide-react";
import type { Rule } from "@/lib/types";
import { RuleEnforcementBadge } from "./RuleEnforcementBadge";
import { EnforcementToggle } from "./EnforcementToggle";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

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
    <>
      <div
        className={`rounded-lg border border-stroke bg-card p-4 text-sm transition duration-200 ${
          isNew ? "animate-fade-in" : ""
        } ${isDeleting ? "opacity-40" : ""}`}
      >
        {isEditing ? (
          <div className="space-y-3">
            <Textarea
              value={editContent}
              onChange={(e) => setEditContent(e.target.value)}
              className="min-h-[72px] resize-none"
              disabled={isSaving}
            />
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <EnforcementToggle
                value={editEnforcement}
                onChange={setEditEnforcement}
                disabled={isSaving}
              />
              <div className="flex justify-end gap-2">
                <Button
                  variant="secondary"
                  onClick={() => setIsEditing(false)}
                  disabled={isSaving}
                  size="sm"
                >
                  <X size={13} /> Cancel
                </Button>
                <Button
                  variant="primary"
                  onClick={handleUpdate}
                  disabled={isSaving || !editContent.trim()}
                  isLoading={isSaving}
                  size="sm"
                >
                  Save
                </Button>
              </div>
            </div>
          </div>
        ) : (
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="whitespace-pre-wrap text-foreground leading-relaxed">{rule.content}</p>
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
        )}
      </div>

      <ConfirmDialog
        isOpen={confirmDelete}
        title="Delete Behavioral Rule"
        description="Are you sure you want to delete this rule? This will stop enforcing this guideline on AI agent workflows in this project."
        confirmText="Delete"
        cancelText="Cancel"
        variant="danger"
        isLoading={isDeleting}
        onConfirm={handleDelete}
        onClose={() => setConfirmDelete(false)}
      />
    </>
  );
}
