"use client";
import { FormEvent, useState, useEffect, useRef } from "react";
import { Loader2, Plus, X, GitBranch, ChevronDown, Sparkles, AlertCircle, Maximize2 } from "lucide-react";
import type { Agent, Repository } from "@/lib/types";
import { TaskMarkdownEditor } from "./TaskMarkdownEditor";
import { AgentSelection } from "./AgentSelection";
import { Field } from "./Field";
import { ComplexitySelection, TaskComplexity } from "./ComplexitySelection";
import { PrioritySelection } from "./PrioritySelection";

export type CreateTaskPayload = {
  title: string;
  description: string;
  complexity: TaskComplexity;
  priority: number;
  labels: string[];
  agent_id?: string;
  repository_id?: string;
};

export function CreateTaskPanel({
  agents,
  repositories,
  isOpen,
  isSubmitting,
  error,
  onClose,
  onSubmit,
}: {
  agents: Agent[];
  repositories: Repository[];
  isOpen: boolean;
  isSubmitting: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (payload: CreateTaskPayload) => Promise<boolean>;
}) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [complexity, setComplexity] = useState<TaskComplexity>("medium");
  const [priority, setPriority] = useState(1);
  const [labels, setLabels] = useState<string[]>([]);
  const [agentID, setAgentID] = useState("");
  const [repositoryID, setRepositoryID] = useState("");
  const [isDescExpanded, setIsDescExpanded] = useState(false);

  const dialogRef = useRef<HTMLDivElement>(null);
  const titleInputRef = useRef<HTMLInputElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Focus trap and Esc key to close
  useEffect(() => {
    if (!isOpen) return;

    // Focus first input on open
    setTimeout(() => {
      titleInputRef.current?.focus();
    }, 50);

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
        return;
      }

      // Basic focus trap
      if (e.key === "Tab" && dialogRef.current) {
        const focusableElements = dialogRef.current.querySelectorAll(
          'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        if (focusableElements.length === 0) return;
        const firstElement = focusableElements[0] as HTMLElement;
        const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

        if (e.shiftKey) {
          if (document.activeElement === firstElement) {
            lastElement.focus();
            e.preventDefault();
          }
        } else {
          if (document.activeElement === lastElement) {
            firstElement.focus();
            e.preventDefault();
          }
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedTitle = title.trim();
    if (!trimmedTitle) return;

    const created = await onSubmit({
      title: trimmedTitle,
      description: description.trim(),
      complexity,
      priority,
      labels,
      agent_id: agentID || undefined,
      repository_id: repositoryID || undefined,
    });
    if (!created) return;

    setTitle("");
    setDescription("");
    setComplexity("medium");
    setPriority(1);
    setLabels([]);
    setAgentID("");
    setRepositoryID("");
  }

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 animate-fade-in"
      role="dialog"
      aria-modal="true"
      aria-labelledby="create-task-title"
      ref={dialogRef}
    >
      {/* Centered Backdrop with blur */}
      <div
        className="absolute inset-0 bg-slate-950/80 backdrop-blur-md transition-opacity duration-300"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Centered Popup Dialog Card (Form wrapper) */}
      <form
        onSubmit={handleSubmit}
        className="animate-modal-in relative z-10 flex w-full max-w-2xl flex-col rounded-xl border border-stroke bg-card shadow-2xl max-h-[90vh] overflow-hidden transition-all duration-300"
      >
        {/* Modal Header */}
        <div className="flex items-center justify-between border-b border-stroke p-5 shrink-0 bg-card/95 backdrop-blur-sm">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-brand-primary-muted border border-brand-primary/20 text-brand-primary shadow-sm">
              <Sparkles size={18} className="animate-pulse" />
            </div>
            <div>
              <h2 id="create-task-title" className="font-sans text-lg font-bold text-foreground tracking-tight">Create New Task</h2>
              <p className="text-xs text-content-muted mt-0.5 font-medium">Define a new task context for execution</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-content-muted transition-all duration-150 hover:bg-surface hover:text-foreground cursor-pointer focus:outline-none focus:ring-2 focus:ring-stroke-focus"
            type="button"
            aria-label="Close modal"
          >
            <X size={18} aria-hidden="true" />
          </button>
        </div>

        {/* Modal Scrollable Content Body */}
        <div className="flex-1 space-y-6 overflow-y-auto p-6 sm:p-8 bg-surface/10 scrollbar-thin">
          <Field label="Title *">
            <input
              ref={titleInputRef}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              className="w-full rounded-lg border border-stroke bg-surface px-4 py-3 text-base text-foreground placeholder-content-muted/40 focus:border-brand-primary focus:ring-2 focus:ring-brand-primary-muted focus:outline-none transition-all duration-150 font-semibold"
              placeholder="e.g. Implement authentication middleware"
              disabled={isSubmitting}
              required
              aria-required="true"
            />
          </Field>

          <Field
            label="Description"
            action={
              <button
                type="button"
                onClick={() => setIsDescExpanded(true)}
                className="inline-flex items-center gap-1 text-[10px] text-brand-primary hover:text-brand-primary/80 cursor-pointer font-sans normal-case font-semibold transition-colors"
                title="Open fullscreen editor with live preview"
              >
                <Maximize2 size={10} />
                Expand Editor
              </button>
            }
          >
            <textarea
              ref={textareaRef}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              className="w-full min-h-[160px] resize-y rounded-lg border border-stroke bg-surface px-4 py-3 text-sm text-foreground placeholder-content-muted/40 focus:border-brand-primary focus:ring-2 focus:ring-brand-primary-muted focus:outline-none transition-all duration-150 scrollbar-thin leading-relaxed"
              placeholder="Detail the target objective, files to modify, or technical requirements."
              disabled={isSubmitting}
            />
          </Field>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Complexity">
              <ComplexitySelection
                complexity={complexity}
                onChange={setComplexity}
                disabled={isSubmitting}
              />
            </Field>

            <Field label="Priority">
              <PrioritySelection
                priority={priority}
                onChange={setPriority}
                disabled={isSubmitting}
              />
            </Field>
          </div>

          <Field label="Repository Context">
            <div className="relative">
              <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-content-muted/60">
                <GitBranch size={16} />
              </div>
              <select
                value={repositoryID}
                onChange={(e) => setRepositoryID(e.target.value)}
                className="w-full pl-9 pr-9 rounded-lg border border-stroke bg-surface py-2.5 text-sm text-foreground focus:border-brand-primary focus:ring-2 focus:ring-brand-primary-muted focus:outline-none transition-all duration-150 appearance-none cursor-pointer font-medium"
                disabled={isSubmitting}
              >
                <option value="">No specific repository</option>
                {repositories.map(repo => (
                  <option key={repo.id} value={repo.id}>
                    {repo.url.split("/").pop()?.replace(".git", "")} ({repo.branch})
                  </option>
                ))}
              </select>
              <div className="absolute inset-y-0 right-0 flex items-center pr-3 pointer-events-none text-content-muted/60">
                <ChevronDown size={14} />
              </div>
            </div>
          </Field>

          <Field label="Assign Agent">
            <AgentSelection agents={agents} agentID={agentID} setAgentID={setAgentID} isSubmitting={isSubmitting} />
          </Field>

          {error && (
            <div className="flex items-start gap-2 rounded-lg border border-red-500/20 bg-red-500/10 p-3 text-xs text-red-200 animate-fade-in" role="alert">
              <AlertCircle size={14} className="shrink-0 mt-0.5 text-red-400" />
              <p className="font-medium">{error}</p>
            </div>
          )}
        </div>

        {/* Modal Footer actions */}
        <div className="flex items-center justify-end gap-3 border-t border-stroke p-5 shrink-0 bg-card/95 backdrop-blur-sm">
          <button
            onClick={onClose}
            className="rounded-lg border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition-all duration-150 hover:bg-surface hover:text-foreground cursor-pointer disabled:opacity-50 focus:outline-none focus:ring-2 focus:ring-stroke-focus"
            disabled={isSubmitting}
            type="button"
          >
            Cancel
          </button>
          <button
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-primary px-5 py-2.5 text-sm font-semibold text-brand-primary-fg hover:opacity-90 hover:shadow-md transition-all duration-150 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-primary-muted"
            disabled={isSubmitting || !title.trim()}
            type="submit"
          >
            {isSubmitting ? <Loader2 size={16} className="animate-spin" aria-hidden="true" /> : <Plus size={16} aria-hidden="true" />}
            {isSubmitting ? "Creating..." : "Create Task"}
          </button>
        </div>
      </form>

      {/* Large Expanded Markdown Editor */}
      {isDescExpanded && (
        <TaskMarkdownEditor
          description={description}
          setDescription={setDescription}
          onClose={() => setIsDescExpanded(false)}
        />
      )}
    </div>
  );
}
