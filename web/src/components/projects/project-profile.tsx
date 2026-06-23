/* eslint-disable react-hooks/set-state-in-effect */
import { FormEvent, useEffect, useState } from "react";
import { Save, Settings, Bot } from "lucide-react";
import { ApiError } from "@/lib/api";
import type { Project } from "@/lib/types";

interface ProjectProfileProps {
  project: Project | undefined;
  onUpdateProject: (input: {
    name?: string;
    description?: string;
    default_model_level?: string;
    default_autonomy?: string;
    auto_review_policy?: string;
    max_retries?: number;
    max_review_fix_cycles?: number;
    default_branch?: string;
  }) => Promise<void>;
}

export function ProjectProfile({ project, onUpdateProject }: ProjectProfileProps) {
  const [name, setName] = useState(project?.name ?? "");
  const [description, setDescription] = useState(project?.description ?? "");
  
  const [defaultModelLevel, setDefaultModelLevel] = useState(project?.default_model_level ?? "balanced");
  const [defaultAutonomy, setDefaultAutonomy] = useState(project?.default_autonomy ?? "supervised");
  const [autoReviewPolicy, setAutoReviewPolicy] = useState(project?.auto_review_policy ?? "complexity_based");
  const [maxRetries, setMaxRetries] = useState(project?.max_retries ?? 3);
  const [maxReviewFixCycles, setMaxReviewFixCycles] = useState(project?.max_review_fix_cycles ?? 3);
  const [defaultBranch, setDefaultBranch] = useState(project?.default_branch ?? "main");

  const [isUpdating, setIsUpdating] = useState(false);
  const [updateError, setUpdateError] = useState("");

  useEffect(() => {
    if (project) {
      setName(project.name);
      setDescription(project.description);
      setDefaultModelLevel(project.default_model_level ?? "balanced");
      setDefaultAutonomy(project.default_autonomy ?? "supervised");
      setAutoReviewPolicy(project.auto_review_policy ?? "complexity_based");
      setMaxRetries(project.max_retries ?? 3);
      setMaxReviewFixCycles(project.max_review_fix_cycles ?? 3);
      setDefaultBranch(project.default_branch ?? "main");
    }
  }, [project]);

  async function handleUpdateProject(e: FormEvent) {
    e.preventDefault();
    setUpdateError("");
    setIsUpdating(true);
    try {
      await onUpdateProject({
        name: name.trim(),
        description: description.trim(),
        default_model_level: defaultModelLevel,
        default_autonomy: defaultAutonomy,
        auto_review_policy: autoReviewPolicy,
        max_retries: maxRetries,
        max_review_fix_cycles: maxReviewFixCycles,
        default_branch: defaultBranch.trim(),
      });
    } catch (err) {
      setUpdateError(err instanceof ApiError ? err.message : "Failed to update project");
    } finally {
      setIsUpdating(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      <form onSubmit={handleUpdateProject} className="space-y-6">
        
        {/* General Settings */}
        <div className="rounded-lg border border-stroke bg-card p-5">
          <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
            <Settings size={18} className="text-brand-primary" />
            <h3 className="font-sans font-semibold text-foreground">General Settings</h3>
          </div>
          <div className="space-y-4">
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Project Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                required
                disabled={isUpdating}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Description</label>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="min-h-[100px] rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none resize-none transition-all"
                disabled={isUpdating}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Default Branch</label>
              <input
                value={defaultBranch}
                onChange={(e) => setDefaultBranch(e.target.value)}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                placeholder="main"
                required
                disabled={isUpdating}
              />
            </div>
          </div>
        </div>

        {/* AI Workflow Defaults */}
        <div className="rounded-lg border border-stroke bg-card p-5">
          <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
            <Bot size={18} className="text-brand-primary" />
            <h3 className="font-sans font-semibold text-foreground">AI Workflow Defaults</h3>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Model Level</label>
              <select
                value={defaultModelLevel}
                onChange={(e) => setDefaultModelLevel(e.target.value)}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                disabled={isUpdating}
              >
                <option value="fast">Fast</option>
                <option value="balanced">Balanced</option>
                <option value="deep">Deep</option>
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Autonomy</label>
              <select
                value={defaultAutonomy}
                onChange={(e) => setDefaultAutonomy(e.target.value)}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                disabled={isUpdating}
              >
                <option value="supervised">Supervised (Requires Approval)</option>
                <option value="autonomous">Autonomous</option>
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Review Policy</label>
              <select
                value={autoReviewPolicy}
                onChange={(e) => setAutoReviewPolicy(e.target.value)}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                disabled={isUpdating}
              >
                <option value="complexity_based">Complexity Based</option>
                <option value="always_review">Always Review</option>
                <option value="auto_merge">Auto Merge (No Review)</option>
              </select>
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Max Retries</label>
              <input
                type="number"
                min={0}
                max={10}
                value={maxRetries}
                onChange={(e) => setMaxRetries(Number(e.target.value))}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                required
                disabled={isUpdating}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Max Review-Fix Cycles</label>
              <input
                type="number"
                min={1}
                max={10}
                value={maxReviewFixCycles}
                onChange={(e) => setMaxReviewFixCycles(Number(e.target.value))}
                className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
                required
                disabled={isUpdating}
              />
            </div>
          </div>
        </div>

        {updateError && <p className="text-xs text-red-400">{updateError}</p>}
        <button
          type="submit"
          disabled={isUpdating}
          className="flex items-center gap-2 rounded bg-brand-primary px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
        >
          <Save size={16} />
          {isUpdating ? "Saving..." : "Save Project Settings"}
        </button>
      </form>
    </div>
  );
}
