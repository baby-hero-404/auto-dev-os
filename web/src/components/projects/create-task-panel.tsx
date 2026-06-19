"use client";

import { FormEvent, useState, useEffect, useRef } from "react";
import { Bot, Loader2, Plus, X } from "lucide-react";
import type { Agent, Repository } from "@/lib/types";

type TaskComplexity = "easy" | "medium" | "hard";

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
  const [labelInput, setLabelInput] = useState("");
  const [agentID, setAgentID] = useState("");
  const [repositoryID, setRepositoryID] = useState("");

  const dialogRef = useRef<HTMLDivElement>(null);
  const titleInputRef = useRef<HTMLInputElement>(null);

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

  const addLabel = () => {
    const trimmed = labelInput.trim().replace(/,/g, "");
    if (trimmed && !labels.includes(trimmed)) {
      setLabels([...labels, trimmed]);
    }
    setLabelInput("");
  };

  const removeLabel = (tagToRemove: string) => {
    setLabels(labels.filter((l) => l !== tagToRemove));
  };

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedTitle = title.trim();
    if (!trimmedTitle) return;

    const finalLabels = [...labels];
    const remainingLabel = labelInput.trim().replace(/,/g, "");
    if (remainingLabel && !finalLabels.includes(remainingLabel)) {
      finalLabels.push(remainingLabel);
    }

    const created = await onSubmit({
      title: trimmedTitle,
      description: description.trim(),
      complexity,
      priority,
      labels: finalLabels,
      agent_id: agentID || undefined,
      repository_id: repositoryID || undefined,
    });
    if (!created) return;

    setTitle("");
    setDescription("");
    setComplexity("medium");
    setPriority(1);
    setLabels([]);
    setLabelInput("");
    setAgentID("");
    setRepositoryID("");
  }

  if (!isOpen) return null;

  const priorities = [
    { label: "Low", value: 1 },
    { label: "Medium", value: 2 },
    { label: "High", value: 3 },
    { label: "Urgent", value: 4 },
  ];

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
        className="absolute inset-0 bg-slate-950/80 backdrop-blur-sm transition-opacity duration-300"
        onClick={onClose}
        aria-hidden="true"
      />
      
      {/* Centered Popup Dialog Card (Form wrapper) */}
      <form 
        onSubmit={handleSubmit}
        className="animate-modal-in relative z-10 flex w-full max-w-lg flex-col rounded-xl border border-stroke bg-card shadow-2xl max-h-[90vh] overflow-hidden"
      >
        {/* Modal Header */}
        <div className="flex items-center justify-between border-b border-stroke p-5 shrink-0 bg-card">
          <div>
            <h2 id="create-task-title" className="font-sans text-lg font-bold text-foreground">Create New Task</h2>
            <p className="text-xs text-content-muted mt-0.5">Define a new task context for execution</p>
          </div>
          <button
            onClick={onClose}
            className="rounded-md p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground cursor-pointer"
            type="button"
            aria-label="Close modal"
          >
            <X size={18} aria-hidden="true" />
          </button>
        </div>

        {/* Modal Scrollable Content Body */}
        <div className="flex-1 space-y-4 overflow-y-auto p-6 bg-surface/5">
          <Field label="Title *">
            <input
              ref={titleInputRef}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              className="w-full rounded-md border border-stroke bg-surface px-3 py-2 text-sm text-foreground placeholder-content-muted/50 focus:border-brand-primary focus:outline-none transition-all"
              placeholder="e.g. Implement authentication middleware"
              disabled={isSubmitting}
              required
              aria-required="true"
            />
          </Field>

          <Field label="Description">
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              className="w-full min-h-[100px] resize-none rounded-md border border-stroke bg-surface px-3 py-2 text-sm text-foreground placeholder-content-muted/50 focus:border-brand-primary focus:outline-none transition-all"
              placeholder="Detail the target objective, files to modify, or technical requirements."
              disabled={isSubmitting}
            />
          </Field>

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="Complexity">
              <div className="grid grid-cols-3 gap-2" role="radiogroup" aria-label="Task complexity">
                {(["easy", "medium", "hard"] as const).map((option) => {
                  const isSelected = complexity === option;
                  const activeColors = {
                    easy: "border-emerald-500/30 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold",
                    medium: "border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold",
                    hard: "border-rose-500/30 bg-rose-500/10 text-rose-600 dark:text-rose-400 font-semibold",
                  };
                  return (
                    <button
                      key={option}
                      onClick={() => setComplexity(option)}
                      role="radio"
                      aria-checked={isSelected}
                      className={`rounded-md border py-2 text-xs capitalize transition cursor-pointer ${
                        isSelected
                          ? activeColors[option]
                          : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground"
                      }`}
                      disabled={isSubmitting}
                      type="button"
                    >
                      {option}
                    </button>
                  );
                })}
              </div>
            </Field>

            <Field label="Priority">
              <div className="grid grid-cols-4 gap-1.5" role="radiogroup" aria-label="Task priority">
                {priorities.map((option) => {
                  const isSelected = priority === option.value;
                  return (
                    <button
                      key={option.value}
                      onClick={() => setPriority(option.value)}
                      role="radio"
                      aria-checked={isSelected}
                      className={`rounded-md border py-2 text-xs transition cursor-pointer ${
                        isSelected
                          ? "border-brand-primary/30 bg-brand-primary/10 text-brand-primary font-semibold"
                          : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground"
                      }`}
                      disabled={isSubmitting}
                      type="button"
                    >
                      {option.label.slice(0, 3)}
                    </button>
                  );
                })}
              </div>
            </Field>
          </div>

          <Field label="Repository Context">
            <select
              value={repositoryID}
              onChange={(e) => setRepositoryID(e.target.value)}
              className="w-full rounded-md border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
              disabled={isSubmitting}
            >
              <option value="">No specific repository</option>
              {repositories.map(repo => (
                <option key={repo.id} value={repo.id}>
                  {repo.url.split("/").pop()?.replace(".git", "")} ({repo.branch})
                </option>
              ))}
            </select>
          </Field>

          <Field label="Labels">
            <div className="space-y-2">
              <div className="flex gap-2">
                <input
                  value={labelInput}
                  onChange={(event) => setLabelInput(event.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === ",") {
                      e.preventDefault();
                      addLabel();
                    }
                  }}
                  className="min-w-0 flex-1 rounded-md border border-stroke bg-surface px-3 py-2 text-sm text-foreground placeholder-content-muted/50 focus:border-brand-primary focus:outline-none transition-all"
                  placeholder="Type label and press Enter"
                  disabled={isSubmitting}
                  aria-label="Add label"
                />
                <button
                  type="button"
                  onClick={addLabel}
                  className="rounded-md border border-stroke bg-surface px-3 py-2 text-xs font-semibold text-foreground hover:bg-surface/80 disabled:opacity-50 cursor-pointer transition-colors"
                  disabled={isSubmitting || !labelInput.trim()}
                >
                  Add
                </button>
              </div>

              {labels.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1" aria-label="Active labels">
                  {labels.map((tag) => (
                    <span
                      key={tag}
                      className="inline-flex items-center gap-1 rounded bg-brand-primary/10 border border-brand-primary/20 px-2 py-0.5 text-xs text-brand-primary font-medium animate-fade-in"
                    >
                      {tag}
                      <button
                        type="button"
                        onClick={() => removeLabel(tag)}
                        className="rounded-full p-0.5 hover:bg-foreground/10 text-brand-primary/70 hover:text-brand-primary cursor-pointer transition-colors"
                        title={`Remove label ${tag}`}
                        aria-label={`Remove label ${tag}`}
                      >
                        <X size={10} aria-hidden="true" />
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </Field>

          <Field label="Assign Agent">
            <div className="space-y-2 max-h-[160px] overflow-y-auto pr-1" role="radiogroup" aria-label="Assign Agent">
              <button
                type="button"
                onClick={() => setAgentID("")}
                role="radio"
                aria-checked={agentID === ""}
                className={`flex w-full items-center gap-3 rounded-lg border p-2.5 text-left text-xs transition-all cursor-pointer ${
                  agentID === ""
                    ? "border-brand-primary/40 bg-brand-primary/10 text-foreground font-semibold"
                    : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground"
                }`}
                disabled={isSubmitting}
              >
                <div className="flex h-7 size-7 shrink-0 items-center justify-center rounded-md bg-card text-brand-primary border border-stroke">
                  <Bot size={14} aria-hidden="true" />
                </div>
                <div>
                  <div className="font-semibold text-foreground">Auto-assign</div>
                  <div className="text-[10px] text-content-muted">Let the system select the best agent</div>
                </div>
              </button>

              {agents.map((agent) => {
                const isSelected = agentID === agent.id;
                const initials = agent.name.split(/\s+/).map(n => n[0]).join("").slice(0, 2).toUpperCase();
                return (
                  <button
                    key={agent.id}
                    type="button"
                    onClick={() => setAgentID(agent.id)}
                    role="radio"
                    aria-checked={isSelected}
                    className={`flex w-full items-center gap-3 rounded-lg border p-2.5 text-left text-xs transition-all cursor-pointer ${
                      isSelected
                        ? "border-brand-primary/40 bg-brand-primary/10 text-foreground font-semibold"
                        : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground"
                    }`}
                    disabled={isSubmitting}
                  >
                    <div className="flex h-7 size-7 shrink-0 items-center justify-center rounded-md bg-card font-mono font-bold text-foreground border border-stroke">
                      {initials}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="truncate font-semibold text-foreground">{agent.name}</div>
                      <div className="truncate text-[10px] text-content-muted">Role: {agent.role} • {agent.model_level_group || "Default Route"}</div>
                    </div>
                  </button>
                );
              })}
            </div>
          </Field>

          {error && <p className="rounded border border-red-500/20 bg-red-550/10 p-2.5 text-xs text-red-200" role="alert">{error}</p>}
        </div>

        {/* Modal Footer actions */}
        <div className="flex items-center justify-end gap-3 border-t border-stroke p-5 shrink-0 bg-card">
          <button
            onClick={onClose}
            className="rounded-md border border-stroke bg-transparent px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
            disabled={isSubmitting}
            type="button"
          >
            Cancel
          </button>
          <button
            className="inline-flex items-center gap-1.5 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 hover:opacity-90 transition cursor-pointer"
            disabled={isSubmitting || !title.trim()}
            type="submit"
          >
            {isSubmitting ? <Loader2 size={16} className="animate-spin" aria-hidden="true" /> : <Plus size={16} aria-hidden="true" />}
            {isSubmitting ? "Creating..." : "Create Task"}
          </button>
        </div>
      </form>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1.5">
      <div className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">{label}</div>
      {children}
    </div>
  );
}
