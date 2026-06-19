"use client";

import { FormEvent, useState } from "react";
import { Check, Edit3, Loader2, Plus, Save, ShieldCheck, Trash2, X } from "lucide-react";
import type { Rule } from "@/lib/types";
import { ApiError } from "@/lib/api";

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
  const [ruleContent, setRuleContent] = useState("");
  const [ruleEnforcement, setRuleEnforcement] = useState<Rule["enforcement"]>("strict");
  const [ruleError, setRuleError] = useState("");
  const [isAddingRule, setIsAddingRule] = useState(false);
  const [editingRuleID, setEditingRuleID] = useState("");
  const [editContent, setEditContent] = useState("");
  const [editEnforcement, setEditEnforcement] = useState<Rule["enforcement"]>("strict");
  const [savingRuleID, setSavingRuleID] = useState("");
  const [savedRuleID, setSavedRuleID] = useState("");
  const [confirmDeleteID, setConfirmDeleteID] = useState("");
  const [deletingRuleID, setDeletingRuleID] = useState("");
  const [newRuleID, setNewRuleID] = useState("");
  const [isSeedingRules, setIsSeedingRules] = useState(false);

  const globalRules = rules.filter((rule) => rule.scope === "global");
  const projectRules = rules.filter((rule) => rule.scope !== "global");

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
      setTimeout(() => setNewRuleID((c) => (c === created.id ? "" : c)), 200);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to add rule");
    } finally {
      setIsAddingRule(false);
    }
  }

  async function handleUpdateRule(rule: Rule) {
    if (savingRuleID) return;
    const content = editContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }
    setRuleError("");
    setSavingRuleID(rule.id);
    try {
      await onUpdateRule(rule.id, content, editEnforcement);
      setSavedRuleID(rule.id);
      setEditingRuleID("");
      setTimeout(() => setSavedRuleID((c) => (c === rule.id ? "" : c)), 2000);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to update rule");
    } finally {
      setSavingRuleID("");
    }
  }

  async function handleDeleteRule(ruleID: string) {
    if (deletingRuleID) return;
    setRuleError("");
    setDeletingRuleID(ruleID);
    try {
      await onDeleteRule(ruleID);
      setConfirmDeleteID("");
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to delete rule");
    } finally {
      setDeletingRuleID("");
    }
  }

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

  function startEdit(rule: Rule) {
    setEditingRuleID(rule.id);
    setEditContent(rule.content);
    setEditEnforcement(rule.enforcement);
    setRuleError("");
    setConfirmDeleteID("");
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
            <button
              onClick={handleSeedRules}
              disabled={isSeedingRules}
              className="inline-flex items-center gap-1.5 rounded border border-stroke bg-surface px-2.5 py-1 text-xs font-semibold text-foreground transition hover:bg-surface/85 cursor-pointer disabled:opacity-50"
              type="button"
            >
              {isSeedingRules ? "Seeding..." : "Auto-seed Default Rules"}
            </button>
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
                projectRules.map((rule) => {
                  const isEditing = editingRuleID === rule.id;
                  const isDeleting = deletingRuleID === rule.id;
                  return (
                    <div
                      key={rule.id}
                      className={`rounded-lg border border-stroke bg-card p-4 text-sm transition duration-200 ${
                        newRuleID === rule.id ? "animate-fade-in" : ""
                      } ${isDeleting ? "opacity-40" : ""}`}
                    >
                      {isEditing ? (
                        <div className="space-y-3">
                          <textarea
                            value={editContent}
                            onChange={(e) => setEditContent(e.target.value)}
                            className="min-h-[72px] w-full resize-none rounded-md border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none"
                            disabled={savingRuleID === rule.id}
                          />
                          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <EnforcementToggle
                              value={editEnforcement}
                              onChange={setEditEnforcement}
                              disabled={savingRuleID === rule.id}
                            />
                            <div className="flex justify-end gap-2">
                              <button
                                onClick={() => setEditingRuleID("")}
                                disabled={savingRuleID === rule.id}
                                className="inline-flex items-center gap-1 rounded border border-stroke px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-surface cursor-pointer disabled:opacity-50"
                                type="button"
                              >
                                <X size={13} /> Cancel
                              </button>
                              <button
                                onClick={() => handleUpdateRule(rule)}
                                disabled={savingRuleID === rule.id || !editContent.trim()}
                                className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1.5 text-xs font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer disabled:opacity-50"
                                type="button"
                              >
                                {savingRuleID === rule.id ? (
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
                                {savedRuleID === rule.id && (
                                  <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-500">
                                    <Check size={13} /> Saved
                                  </span>
                                )}
                              </div>
                            </div>
                            <div className="flex shrink-0 gap-1.5">
                              <button
                                onClick={() => startEdit(rule)}
                                className="rounded p-1 text-content-muted hover:bg-surface hover:text-foreground transition cursor-pointer"
                                title="Edit rule"
                                type="button"
                              >
                                <Edit3 size={14} />
                              </button>
                              <button
                                onClick={() => setConfirmDeleteID(rule.id)}
                                className="rounded p-1 text-content-muted hover:bg-rose-500/10 hover:text-rose-500 transition cursor-pointer"
                                title="Delete rule"
                                type="button"
                              >
                                <Trash2 size={14} />
                              </button>
                            </div>
                          </div>

                          {confirmDeleteID === rule.id && (
                            <div className="mt-3 flex flex-col gap-2 rounded-md border border-rose-500/20 bg-rose-500/5 p-3 sm:flex-row sm:items-center sm:justify-between">
                              <p className="text-xs font-semibold text-rose-800 dark:text-rose-200">
                                Are you sure you want to delete this rule?
                              </p>
                              <div className="flex gap-2">
                                <button
                                  onClick={() => handleDeleteRule(rule.id)}
                                  disabled={deletingRuleID === rule.id}
                                  className="inline-flex items-center gap-1 rounded bg-rose-500 px-3 py-1 text-xs font-semibold text-white hover:opacity-90 cursor-pointer"
                                  type="button"
                                >
                                  {deletingRuleID === rule.id && <Loader2 size={12} className="animate-spin" />} Delete
                                </button>
                                <button
                                  onClick={() => setConfirmDeleteID("")}
                                  disabled={deletingRuleID === rule.id}
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
                })
              )}
            </div>
          </div>
        )}
      </div>

      {/* Add Project Rule Form Card */}
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

          {ruleError && <p className="text-xs text-red-450">{ruleError}</p>}

          <button
            type="submit"
            disabled={isAddingRule || !ruleContent.trim()}
            className="flex w-full items-center justify-center gap-2 rounded bg-brand-primary px-3 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
          >
            {isAddingRule ? <Loader2 size={14} className="animate-spin" /> : <Plus size={14} />} Add Rule
          </button>
        </form>
      </div>
    </div>
  );
}

function RuleEnforcementBadge({ enforcement }: { enforcement: Rule["enforcement"] }) {
  const cls =
    enforcement === "strict"
      ? "border-rose-500/20 bg-rose-500/10 text-rose-600 dark:text-rose-400"
      : "border-amber-500/20 bg-amber-500/10 text-amber-600 dark:text-amber-400";
  return (
    <span className={`rounded border px-2 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider ${cls}`}>
      {enforcement}
    </span>
  );
}

function EnforcementToggle({
  value,
  onChange,
  disabled,
}: {
  value: Rule["enforcement"];
  onChange: (v: Rule["enforcement"]) => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <span className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
        Enforcement
      </span>
      {(["strict", "advisory"] as const).map((opt) => (
        <button
          key={opt}
          onClick={() => onChange(opt)}
          disabled={disabled}
          className={`inline-flex items-center gap-1.5 rounded-md border px-3 py-1.5 text-xs font-semibold transition disabled:opacity-50 cursor-pointer ${
            value === opt
              ? opt === "strict"
                ? "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-400 font-semibold"
                : "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-400 font-semibold"
              : "border-stroke bg-card text-content-muted hover:text-foreground"
          }`}
          type="button"
        >
          <span className={`size-1.5 rounded-full ${value === opt ? "bg-current animate-pulse" : "bg-slate-300 dark:bg-slate-700"}`} />
          {opt}
        </button>
      ))}
    </div>
  );
}

function RulesSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-card p-4">
          <div className="skeleton-shimmer h-4 w-5/6 rounded" />
          <div className="mt-3 flex gap-2">
            <div className="skeleton-shimmer h-5 w-16 rounded" />
            <div className="skeleton-shimmer h-5 w-14 rounded" />
          </div>
        </div>
      ))}
    </div>
  );
}
