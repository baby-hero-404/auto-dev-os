import { ChevronRight, Database, Search, Zap } from "lucide-react";
import type { Skill } from "@/lib/types";
import { parseSkillMeta } from "../utils";
import { CategoryChip, EmptyState, LoadingState, PanelHeader, Tag } from "./CommonUI";

interface CatalogPanelProps {
  skills: Skill[];
  selectedSkillID: string;
  searchQuery: string;
  categories: string[];
  selectedCategory: string;
  totalSkills: number;
  isLoading: boolean;
  hasSource: boolean;
  onSearchChange: (value: string) => void;
  onCategoryChange: (value: string) => void;
  onSelectSkill: (skill: Skill) => void;
}

export function CatalogPanel({
  skills,
  selectedSkillID,
  searchQuery,
  categories,
  selectedCategory,
  totalSkills,
  isLoading,
  hasSource,
  onSearchChange,
  onCategoryChange,
  onSelectSkill,
}: CatalogPanelProps) {
  return (
    <section className="flex max-h-[calc(100vh-220px)] min-h-[620px] flex-col rounded-lg border border-stroke bg-card p-4">
      <div className="flex items-start justify-between border-b border-stroke pb-3 mb-4">
        <PanelHeader icon={Zap} title="Catalog" detail={`${skills.length} of ${totalSkills} skills`} />
        <span className="rounded-full border border-stroke bg-surface/60 px-2 py-0.5 text-[9px] font-semibold uppercase tracking-wide text-content-muted">
          Read Only
        </span>
      </div>

      <div className="mb-2 flex items-center justify-between gap-3">
        <div>
          <div className="text-[10px] uppercase tracking-wider text-content-muted">Catalog search</div>
          <div className="text-xs text-content-muted">Search only the connected skills repository.</div>
        </div>
      </div>

      <div className="relative mb-3">
        <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-content-muted" />
        <input
          value={searchQuery}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Search skills, categories, or registry entries"
          className="w-full rounded-md border border-stroke bg-background py-2 pl-9 pr-3 text-sm text-foreground outline-none transition focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20"
        />
      </div>

      <div className="mb-3 flex gap-1.5 overflow-x-auto pb-1">
        <CategoryChip active={selectedCategory === "all"} onClick={() => onCategoryChange("all")}>
          All
        </CategoryChip>
        {categories.map((category) => (
          <CategoryChip key={category} active={selectedCategory === category} onClick={() => onCategoryChange(category)}>
            {category}
          </CategoryChip>
        ))}
      </div>

      <div className="min-h-0 flex-1 space-y-2 overflow-y-auto pr-1">
        {isLoading ? (
          <LoadingState label="Loading skills" />
        ) : skills.length > 0 ? (
          skills.map((skill) => {
            const meta = parseSkillMeta(skill);
            const isSelected = selectedSkillID === skill.id;
            return (
              <button
                key={skill.id}
                type="button"
                onClick={() => onSelectSkill(skill)}
                className={`w-full rounded-md border p-3 text-left transition-all duration-200 ${
                  isSelected
                    ? "border-brand-primary border-l-4 border-l-brand-primary pl-4 bg-brand-primary/12 shadow-sm ring-1 ring-brand-primary/15"
                    : "cursor-pointer border-stroke bg-surface/25 hover:border-brand-primary/50 hover:bg-surface/60 hover:pl-4"
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <span className="min-w-0 truncate font-mono text-sm font-semibold text-foreground">{skill.name}</span>
                  <div className="flex items-center gap-2">
                    {isSelected && (
                      <span className="rounded-full border border-brand-primary/20 bg-brand-primary/10 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-brand-primary">
                        Selected
                      </span>
                    )}
                    <ChevronRight size={14} className="mt-0.5 shrink-0 text-content-muted" />
                  </div>
                </div>
                <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-content-muted/90">
                  {skill.description || "No description provided."}
                </p>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  <Tag>{meta.category}</Tag>
                  {meta.source !== "registry" && <Tag>{meta.source}</Tag>}
                </div>
              </button>
            );
          })
        ) : (
          <div className="rounded-md border border-dashed border-stroke bg-surface/20 p-5">
            <EmptyState
              icon={Database}
              title={hasSource ? "No skills found" : "No repository connected"}
              description={
                hasSource
                  ? "Try a different search term, then sync the repository again."
                  : "Connect a valid Git repository URL and sync it. The repo root must contain registry.json or registry.min.json."
              }
            />
            {!hasSource && (
              <div className="mt-4 grid gap-2 text-xs text-content-muted">
                <div className="rounded-md border border-stroke bg-background px-3 py-2">1. Paste a Git URL in the connection bar above.</div>
                <div className="rounded-md border border-stroke bg-background px-3 py-2">2. Sync the repo and confirm the root manifest exists.</div>
                <div className="rounded-md border border-stroke bg-background px-3 py-2">3. Open a skill to inspect metadata and source files.</div>
              </div>
            )}
          </div>
        )}
      </div>
    </section>
  );
}
