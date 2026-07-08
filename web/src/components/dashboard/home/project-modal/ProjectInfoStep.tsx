"use client";

import { FormEvent } from "react";
import { ArrowLeft } from "lucide-react";

interface ProjectInfoStepProps {
  projectName: string;
  setProjectName: (val: string) => void;
  projectDescription: string;
  setProjectDescription: (val: string) => void;
  isSubmitting: boolean;
  creationError: string;
  onNext: (e: FormEvent) => void;
  onCancel: () => void;
}

export function ProjectInfoStep({
  projectName,
  setProjectName,
  projectDescription,
  setProjectDescription,
  isSubmitting,
  creationError,
  onNext,
  onCancel,
}: ProjectInfoStepProps) {
  return (
    <form className="mt-4 flex flex-col gap-4" onSubmit={onNext}>
      <div className="flex flex-col gap-1.5">
        <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">
          Name <span className="text-brand-primary">*</span>
        </label>
        <input
          value={projectName}
          onChange={(e) => setProjectName(e.target.value)}
          placeholder="e.g. api-backend"
          className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition"
          disabled={isSubmitting}
          required
          autoFocus
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <label className="text-xs font-semibold uppercase tracking-wider text-content-muted">
          Description
        </label>
        <textarea
          value={projectDescription}
          onChange={(e) => setProjectDescription(e.target.value)}
          placeholder="Optional goal, scope, or repository context."
          className="min-h-[100px] rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary transition resize-none"
          disabled={isSubmitting}
        />
      </div>

      {creationError && (
        <p className="rounded-md border border-red-500/20 bg-red-950/40 p-3 text-xs text-red-200">
          {creationError}
        </p>
      )}

      <div className="mt-2 flex items-center justify-end gap-3 border-t border-stroke pt-4">
        <button
          onClick={onCancel}
          className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
          disabled={isSubmitting}
          type="button"
        >
          Cancel
        </button>
        <button
          className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 cursor-pointer disabled:opacity-50"
          disabled={isSubmitting}
          type="submit"
        >
          Next
          <ArrowLeft size={16} className="rotate-180" />
        </button>
      </div>
    </form>
  );
}
