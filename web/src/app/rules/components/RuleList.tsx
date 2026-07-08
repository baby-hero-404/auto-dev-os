import { Search, ShieldCheck, Sparkles, X } from "lucide-react";
import type { Rule } from "@/lib/types";
import { RuleListItem } from "./RuleListItem";
import { RulesSkeleton } from "./RuleUI";

export function RuleList({
  rules,
  isLoading,
  searchQuery,
  onSearchChange,
  onSeedRules,
  isSeeding,
  onOpenAddModal,
  newRuleID,
  onUpdateRule,
  onDeleteRule,
  token,
}: {
  rules: Rule[];
  isLoading: boolean;
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onSeedRules: () => void;
  isSeeding: boolean;
  onOpenAddModal: () => void;
  newRuleID: string;
  onUpdateRule: (rule: Rule) => void;
  onDeleteRule: (id: string) => void;
  token: string;
}) {
  const filteredRules = rules.filter((rule) =>
    rule.content.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
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
                onChange={(e) => onSearchChange(e.target.value)}
                className="w-full rounded-lg border border-stroke bg-background/50 pl-9 pr-8 py-1.5 text-xs text-foreground placeholder:text-content-muted/70 focus:border-brand-primary focus:outline-none focus:ring-1 focus:ring-brand-primary/20 transition-all duration-200"
              />
              {searchQuery && (
                <button
                  onClick={() => onSearchChange("")}
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
              onClick={onSeedRules}
              disabled={isSeeding}
              className="inline-flex items-center justify-center gap-1.5 rounded-lg border border-stroke bg-surface px-3 py-1.5 text-xs font-semibold text-foreground transition hover:bg-panel-muted cursor-pointer disabled:opacity-50"
              type="button"
            >
              <Sparkles size={13} className="text-brand-primary" />
              {isSeeding ? "Filling..." : "Fill Default Rules"}
            </button>
          )}
        </div>
      </div>

      {isLoading ? (
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
              onClick={onOpenAddModal}
              className="inline-flex items-center gap-2 rounded-lg bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 shadow-sm transition hover:opacity-90 cursor-pointer"
              type="button"
            >
              <ShieldCheck size={15} />
              Add Global Rule
            </button>
            <button
              onClick={onSeedRules}
              disabled={isSeeding}
              className="inline-flex items-center gap-2 rounded-lg border border-stroke bg-card px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
              type="button"
            >
              <Sparkles size={15} />
              {isSeeding ? "Filling..." : "Fill Default Rules"}
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
          {filteredRules.map((rule) => (
            <RuleListItem
              key={rule.id}
              rule={rule}
              token={token}
              isNew={newRuleID === rule.id}
              onUpdate={onUpdateRule}
              onDelete={onDeleteRule}
            />
          ))}
        </div>
      )}
    </div>
  );
}
