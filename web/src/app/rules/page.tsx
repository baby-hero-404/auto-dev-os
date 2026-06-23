"use client";

import { useState, FormEvent } from "react";
import useSWR from "swr";
import { ShieldCheck, Plus, Edit3, Trash2, Loader2, Save, X, Check, Sparkles, Search, ShieldAlert, Info } from "lucide-react";
import { toast, Toaster } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import type { Rule } from "@/lib/types";

const DEFAULT_GLOBAL_RULES: Array<{ content: string; enforcement: Rule["enforcement"] }> = [
  {
    content: "Follow clean code principles: self-documenting code, meaningful variable names, small focused functions.",
    enforcement: "strict",
  },
  {
    content: "All code changes must include tests. No PR may be merged without passing CI.",
    enforcement: "strict",
  },
  {
    content: "Use conventional commit messages: feat:, fix:, docs:, refactor:, test:, chore:.",
    enforcement: "advisory",
  },
  {
    content: "Security first: never log secrets, validate all inputs, use parameterized queries.",
    enforcement: "strict",
  },
  {
    content: "Document architectural decisions in ADRs. Update ARCHITECTURE.md when adding new packages or changing data flow.",
    enforcement: "advisory",
  },
  {
    content: "Strictly enforce the Socratic Gate (Definition of Ready): before starting implementation on any Medium/Hard tasks, ask the user at least 3 strategic questions to clarify specifications and boundary conditions. Do not start coding until requirements are explicitly confirmed.",
    enforcement: "strict",
  },
  {
    content: "Ensure all code edits are surgical and targeted. Modify only the necessary parts of the codebase, preserving surrounding code style, docstrings, and comments.",
    enforcement: "strict",
  },
  {
    content: "Practice Progressive Discovery and JIT Knowledge: read specific line ranges rather than loading entire files. Dynamically load/unload task-specific skills and remove them from context once the subtask is complete to avoid context window overflow.",
    enforcement: "strict",
  },
  {
    content: "Always perform self-checks and verify your implementation by running local tests and linting before marking a task as complete.",
    enforcement: "strict",
  },
];

export default function RulesPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: rules = [], mutate: mutateRules, isLoading: isRulesLoading } = useSWR(
    orgID && token ? ["global-rules", orgID] : null,
    () => api.listGlobalRules(orgID, token),
  );

  // Form states
  const [ruleContent, setRuleContent] = useState("");
  const [ruleEnforcement, setRuleEnforcement] = useState<Rule["enforcement"]>("strict");
  const [isAddingRule, setIsAddingRule] = useState(false);
  const [ruleError, setRuleError] = useState("");
  const [isSeedingRules, setIsSeedingRules] = useState(false);
  const [isAddModalOpen, setIsAddModalOpen] = useState(false);

  // Edit states
  const [editingRuleID, setEditingRuleID] = useState("");
  const [editRuleContent, setEditRuleContent] = useState("");
  const [editRuleEnforcement, setEditRuleEnforcement] = useState<Rule["enforcement"]>("strict");
  const [savingRuleID, setSavingRuleID] = useState("");
  const [savedRuleID, setSavedRuleID] = useState("");

  // Delete states
  const [confirmingDeleteRuleID, setConfirmingDeleteRuleID] = useState("");
  const [deletingRuleID, setDeletingRuleID] = useState("");

  const [newRuleID, setNewRuleID] = useState("");
  const [searchQuery, setSearchQuery] = useState("");

  const filteredRules = rules.filter((rule) =>
    rule.content.toLowerCase().includes(searchQuery.toLowerCase())
  );

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
      setRuleContent("");
      setRuleEnforcement("strict");
      setIsAddModalOpen(false);
      setNewRuleID(createdRule.id);
      mutateRules([createdRule, ...rules], false);
      toast.success("Global rule added successfully!");
      window.setTimeout(() => {
        setNewRuleID((current) => (current === createdRule.id ? "" : current));
      }, 2000);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to add rule");
    } finally {
      setIsAddingRule(false);
    }
  }

  function openAddModal() {
    setRuleContent("");
    setRuleEnforcement("strict");
    setRuleError("");
    setIsAddModalOpen(true);
  }

  function closeAddModal() {
    if (isAddingRule) return;
    setIsAddModalOpen(false);
    setRuleError("");
  }

  async function handleSeedRules() {
    if (!orgID || !token || isSeedingRules) return;
    setRuleError("");
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

  function startEditRule(rule: Rule) {
    setEditingRuleID(rule.id);
    setEditRuleContent(rule.content);
    setEditRuleEnforcement(rule.enforcement);
    setRuleError("");
    setConfirmingDeleteRuleID("");
  }

  function cancelEditRule() {
    setEditingRuleID("");
    setEditRuleContent("");
    setEditRuleEnforcement("strict");
  }

  async function handleUpdateRule(rule: Rule) {
    if (!token || savingRuleID) return;
    const content = editRuleContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }

    setRuleError("");
    setSavingRuleID(rule.id);
    try {
      const updatedRule = await api.updateRule(rule.id, token, {
        content,
        enforcement: editRuleEnforcement,
      });
      mutateRules(rules.map((item) => (item.id === rule.id ? updatedRule : item)), false);
      setSavedRuleID(rule.id);
      setEditingRuleID("");
      toast.success("Rule updated successfully!");
      window.setTimeout(() => {
        setSavedRuleID((current) => (current === rule.id ? "" : current));
      }, 2000);
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to update rule");
    } finally {
      setSavingRuleID("");
    }
  }

  async function handleDeleteRule(ruleID: string) {
    if (!token || deletingRuleID) return;

    setRuleError("");
    setDeletingRuleID(ruleID);
    try {
      await api.deleteRule(ruleID, token);
      mutateRules(rules.filter((rule) => rule.id !== ruleID), false);
      setConfirmingDeleteRuleID("");
      toast.success("Rule deleted successfully!");
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to delete rule");
    } finally {
      setDeletingRuleID("");
    }
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
            onClick={openAddModal}
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
            {/* Rules list */}
            <div className="rounded-xl border border-stroke bg-card p-6 shadow-md transition duration-200">
              <div className="mb-5 flex flex-col gap-4 border-b border-stroke pb-4 sm:flex-row sm:items-center sm:justify-between">
                <div className="flex items-center gap-2.5">
                  <ShieldCheck size={20} className="text-brand-primary" />
                  <h3 className="font-mono text-base font-bold text-foreground">Active Guardrails</h3>
                  {rules.length > 0 && (
                    <span className="rounded-full border border-stroke bg-surface px-2.5 py-0.5 text-xs font-semibold text-content-muted/90 shadow-xs">
                      {rules.length} total
                    </span>
                  )}
                </div>

                <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:w-auto w-full">
                  {/* Search box */}
                  {rules.length > 0 && (
                    <div className="relative w-full sm:w-64">
                      <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-content-muted" />
                      <input
                        type="text"
                        placeholder="Search rules..."
                        value={searchQuery}
                        onChange={(e) => setSearchQuery(e.target.value)}
                        className="w-full rounded-lg border border-stroke bg-background/50 pl-9 pr-8 py-1.5 text-xs text-foreground placeholder:text-content-muted/70 focus:border-brand-primary focus:outline-none focus:ring-1 focus:ring-brand-primary/20 transition-all duration-200"
                      />
                      {searchQuery && (
                        <button
                          onClick={() => setSearchQuery("")}
                          className="absolute right-2.5 top-1/2 -translate-y-1/2 text-content-muted hover:text-foreground cursor-pointer"
                          type="button"
                        >
                          <X size={12} />
                        </button>
                      )}
                    </div>
                  )}

                  {rules.length === 0 && (
                    <button
                      onClick={handleSeedRules}
                      disabled={isSeedingRules}
                      className="inline-flex items-center justify-center gap-1.5 rounded-lg border border-stroke bg-surface px-3 py-1.5 text-xs font-semibold text-foreground transition hover:bg-panel-muted cursor-pointer disabled:opacity-50"
                      type="button"
                    >
                      <Sparkles size={13} className="text-brand-primary" />
                      {isSeedingRules ? "Filling..." : "Fill Default Rules"}
                    </button>
                  )}
                </div>
              </div>

              {isRulesLoading ? (
                <RulesSkeleton />
              ) : rules.length === 0 ? (
                <div className="rounded-xl border border-dashed border-stroke bg-panel/10 p-10 text-center">
                  <ShieldCheck size={40} className="mx-auto text-content-muted/40 mb-3" />
                  <p className="font-mono text-base font-bold text-foreground">No global rules configured yet.</p>
                  <p className="mt-2 text-sm text-content-muted max-w-sm mx-auto leading-relaxed">
                    Add organization-wide constraints that every project and agent must follow to guarantee clean code and consistency.
                  </p>
                  <div className="mt-6 flex flex-wrap justify-center gap-3">
                    <button
                      onClick={openAddModal}
                      className="inline-flex items-center gap-2 rounded-lg bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 shadow-sm transition hover:opacity-90 cursor-pointer"
                      type="button"
                    >
                      <Plus size={15} />
                      Add Global Rule
                    </button>
                    <button
                      onClick={handleSeedRules}
                      disabled={isSeedingRules}
                      className="inline-flex items-center gap-2 rounded-lg border border-stroke bg-card px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                      type="button"
                    >
                      <Sparkles size={15} />
                      {isSeedingRules ? "Filling..." : "Fill Default Rules"}
                    </button>
                  </div>
                </div>
              ) : filteredRules.length === 0 ? (
                <div className="rounded-xl border border-dashed border-stroke bg-panel/10 p-10 text-center">
                  <Search size={32} className="mx-auto text-content-muted/40 mb-2" />
                  <p className="font-mono text-sm font-semibold text-foreground">No matching rules found</p>
                  <p className="mt-1 text-xs text-content-muted">
                    Try adjusting your search query to find the global rule.
                  </p>
                </div>
              ) : (
                <div className="space-y-3 max-h-[600px] overflow-y-auto pr-1">
                  {filteredRules.map((rule) => {
                    const isEditing = editingRuleID === rule.id;
                    const isDeleting = deletingRuleID === rule.id;
                    const isStrict = rule.enforcement === "strict";
                    return (
                      <div
                        key={rule.id}
                        className={`group rounded-lg border border-stroke bg-panel/30 p-4.5 text-sm transition-all duration-200 hover:bg-panel/60 hover:shadow-xs ${
                          isStrict 
                            ? "border-l-4 border-l-rose-500/80" 
                            : "border-l-4 border-l-amber-500/80"
                        } ${
                          newRuleID === rule.id ? "ring-2 ring-brand-primary bg-brand-primary/5 border-brand-primary" : ""
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
                                disabled={savingRuleID === rule.id}
                              />
                              <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                                <EnforcementToggle
                                  value={editRuleEnforcement}
                                  onChange={setEditRuleEnforcement}
                                  disabled={savingRuleID === rule.id}
                                />
                                <div className="flex justify-end gap-2">
                                  <button
                                    onClick={cancelEditRule}
                                    disabled={savingRuleID === rule.id}
                                    className="inline-flex items-center gap-1.5 rounded-lg border border-stroke px-3 py-1.5 text-xs font-semibold text-foreground transition hover:bg-surface disabled:opacity-50 cursor-pointer"
                                    type="button"
                                  >
                                    <X size={13} />
                                    Cancel
                                  </button>
                                  <button
                                    onClick={() => handleUpdateRule(rule)}
                                    disabled={savingRuleID === rule.id || !editRuleContent.trim()}
                                    className="inline-flex items-center gap-1.5 rounded-lg bg-brand-primary px-3 py-1.5 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                                    type="button"
                                  >
                                    {savingRuleID === rule.id ? <Loader2 size={13} className="animate-spin" /> : <Save size={13} />}
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
                                {/* Enforcement Icon Banner */}
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
                                    {savedRuleID === rule.id && (
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
                                  onClick={() => startEditRule(rule)}
                                  className="rounded-lg p-2 text-content-muted transition hover:bg-surface hover:text-foreground cursor-pointer"
                                  title="Edit rule"
                                  type="button"
                                >
                                  <Edit3 size={14} />
                                </button>
                                <button
                                  onClick={() => setConfirmingDeleteRuleID(rule.id)}
                                  className="rounded-lg p-2 text-content-muted transition hover:bg-rose-500/10 hover:text-rose-600 dark:hover:text-rose-400 cursor-pointer"
                                  title="Delete rule"
                                  type="button"
                                >
                                  <Trash2 size={14} />
                                </button>
                              </div>
                            </div>
                            {confirmingDeleteRuleID === rule.id && (
                              <div className="mt-3 flex flex-col gap-2 rounded-lg border border-red-500/20 bg-red-500/5 p-3 sm:flex-row sm:items-center sm:justify-between">
                                <p className="text-xs font-semibold text-red-700 dark:text-red-300">Are you sure you want to delete this rule?</p>
                                <div className="flex gap-2">
                                  <button
                                    onClick={() => handleDeleteRule(rule.id)}
                                    disabled={isDeleting}
                                    className="inline-flex items-center gap-1 rounded bg-red-500 px-3 py-1 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                                    type="button"
                                  >
                                    {isDeleting && <Loader2 size={12} className="animate-spin" />}
                                    Delete
                                  </button>
                                  <button
                                    onClick={() => setConfirmingDeleteRuleID("")}
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
                  })}
                </div>
              )}
            </div>

        </div>
      </div>
      {isAddModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 p-4 backdrop-blur-xs">
          <div className="relative w-full max-w-lg rounded-xl border border-stroke bg-card p-6 shadow-2xl animate-in fade-in zoom-in duration-200">
            <button
              onClick={closeAddModal}
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
                  onClick={closeAddModal}
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
      )}
    </DashboardLayout>
  );
}

function RuleEnforcementBadge({ enforcement }: { enforcement: Rule["enforcement"] }) {
  const classes = enforcement === "strict"
    ? "border-rose-500/20 bg-rose-500/5 text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300"
    : "border-amber-500/20 bg-amber-500/5 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300";

  return (
    <span className={`rounded-full border px-2.5 py-0.5 font-mono text-[9px] font-bold uppercase tracking-wider shadow-sm ${classes}`}>
      {enforcement}
    </span>
  );
}

function RulesSkeleton() {
  return (
    <div className="space-y-2">
      {[0, 1, 2].map((i) => (
        <div key={i} className="rounded-lg border border-stroke bg-panel p-4">
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

function EnforcementToggle({
  value,
  onChange,
  disabled,
  showDescriptions = false,
}: {
  value: Rule["enforcement"];
  onChange: (val: Rule["enforcement"]) => void;
  disabled?: boolean;
  showDescriptions?: boolean;
}) {
  const options: Array<{ value: Rule["enforcement"]; label: string; description: string }> = [
    {
      value: "strict",
      label: "Strict",
      description: "Required guardrail. Agents must follow it and should refuse work that conflicts with it.",
    },
    {
      value: "advisory",
      label: "Advisory",
      description: "Guidance preference. Agents should follow it when possible, but it can yield to task context.",
    },
  ];

  return (
    <div className={`grid gap-2 ${showDescriptions ? "sm:grid-cols-2" : "grid-cols-2"}`}>
      {options.map((option) => {
        const isSelected = value === option.value;
        const selectedClasses = option.value === "strict"
          ? "border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-200"
          : "border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-200";
        return (
          <button
            key={option.value}
            onClick={() => onChange(option.value)}
            disabled={disabled}
            className={`rounded border px-3 py-2 text-left transition cursor-pointer disabled:opacity-50 ${
              isSelected ? selectedClasses : "border-stroke bg-background text-content-muted hover:text-foreground"
            }`}
            type="button"
          >
            <span className="block text-xs font-semibold">{option.label}</span>
            {showDescriptions && (
              <span className="mt-1 block text-xs leading-5 text-content-muted">{option.description}</span>
            )}
          </button>
        );
      })}
    </div>
  );
}
