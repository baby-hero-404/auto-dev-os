"use client";
import { FormEvent, useState, useEffect, useRef } from "react";
import { Bot, Loader2, Plus, X, GitBranch, ChevronDown, Tag, Sparkles, Check, AlertCircle, FileText, Minimize2, Maximize2 } from "lucide-react";
import type { Agent, Repository } from "@/lib/types";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

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
        className="absolute inset-0 bg-slate-950/80 backdrop-blur-md transition-opacity duration-300"
        onClick={onClose}
        aria-hidden="true"
      />
      
      {/* Centered Popup Dialog Card (Form wrapper) */}
      <form 
        onSubmit={handleSubmit}
        className="animate-modal-in relative z-10 flex w-full max-w-lg flex-col rounded-xl border border-stroke bg-card shadow-2xl max-h-[90vh] overflow-hidden transition-all duration-300"
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
        <div className="flex-1 space-y-5 overflow-y-auto p-6 bg-surface/10 scrollbar-thin">
          <Field label="Title *">
            <input
              ref={titleInputRef}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              className="w-full rounded-lg border border-stroke bg-surface px-3 py-2.5 text-sm text-foreground placeholder-content-muted/40 focus:border-brand-primary focus:ring-2 focus:ring-brand-primary-muted focus:outline-none transition-all duration-150 font-medium"
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
              className="w-full min-h-[100px] resize-y rounded-lg border border-stroke bg-surface px-3 py-2.5 text-sm text-foreground placeholder-content-muted/40 focus:border-brand-primary focus:ring-2 focus:ring-brand-primary-muted focus:outline-none transition-all duration-150 scrollbar-thin"
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
                    easy: "border-emerald-500/40 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold shadow-sm",
                    medium: "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold shadow-sm",
                    hard: "border-rose-500/40 bg-rose-500/10 text-rose-600 dark:text-rose-400 font-semibold shadow-sm",
                  };
                  const dotColors = {
                    easy: "bg-emerald-500",
                    medium: "bg-amber-500",
                    hard: "bg-rose-500",
                  };
                  return (
                    <button
                      key={option}
                      onClick={() => setComplexity(option)}
                      role="radio"
                      aria-checked={isSelected}
                      className={`flex items-center justify-center gap-1.5 rounded-lg border py-2 text-xs capitalize transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus ${
                        isSelected
                          ? activeColors[option]
                          : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
                      }`}
                      disabled={isSubmitting}
                      type="button"
                    >
                      <span className={`h-1.5 w-1.5 rounded-full ${dotColors[option]}`} />
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
                      className={`rounded-lg border py-2 text-xs transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus font-medium ${
                        isSelected
                          ? "border-brand-primary/40 bg-brand-primary-muted text-brand-primary font-semibold shadow-sm"
                          : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
                      }`}
                      disabled={isSubmitting}
                      type="button"
                    >
                      {option.label}
                    </button>
                  );
                })}
              </div>
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

          <Field label="Labels">
            <div className="space-y-2">
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none text-content-muted/60">
                    <Tag size={14} />
                  </div>
                  <input
                    value={labelInput}
                    onChange={(event) => setLabelInput(event.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === ",") {
                        e.preventDefault();
                        addLabel();
                      }
                    }}
                    className="w-full pl-9 pr-3 rounded-lg border border-stroke bg-surface py-2 text-sm text-foreground placeholder-content-muted/40 focus:border-brand-primary focus:ring-2 focus:ring-brand-primary-muted focus:outline-none transition-all duration-150"
                    placeholder="Type label and press Enter"
                    disabled={isSubmitting}
                    aria-label="Add label"
                  />
                </div>
                <button
                  type="button"
                  onClick={addLabel}
                  className="rounded-lg border border-stroke bg-surface px-4 py-2 text-xs font-semibold text-foreground hover:bg-surface-code hover:border-stroke-focus disabled:opacity-50 cursor-pointer transition-all duration-150 focus:outline-none focus:ring-1 focus:ring-stroke-focus"
                  disabled={isSubmitting || !labelInput.trim()}
                >
                  Add
                </button>
              </div>

              {labels.length > 0 && (
                <div className="flex flex-wrap gap-1.5 pt-1.5" aria-label="Active labels">
                  {labels.map((tag) => (
                    <span
                      key={tag}
                      className="inline-flex items-center gap-1 rounded-md bg-brand-primary-muted border border-brand-primary/20 px-2 py-0.5 text-xs text-brand-primary font-semibold animate-fade-in"
                    >
                      {tag}
                      <button
                        type="button"
                        onClick={() => removeLabel(tag)}
                        className="rounded-full p-0.5 hover:bg-brand-primary/20 text-brand-primary/70 hover:text-brand-primary cursor-pointer transition-colors"
                        title={`Remove label ${tag}`}
                        aria-label={`Remove label ${tag}`}
                      >
                        <X size={11} aria-hidden="true" />
                      </button>
                    </span>
                  ))}
                </div>
              )}
            </div>
          </Field>

          <Field label="Assign Agent">
            <div className="space-y-2 max-h-[175px] overflow-y-auto pr-1.5 scrollbar-thin flex flex-col gap-1.5" role="radiogroup" aria-label="Assign Agent">
              <button
                type="button"
                onClick={() => setAgentID("")}
                role="radio"
                aria-checked={agentID === ""}
                className={`flex w-full items-center gap-3.5 rounded-lg border p-3 text-left text-xs transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus ${
                  agentID === ""
                    ? "border-brand-primary/45 bg-brand-primary-muted text-foreground font-semibold shadow-sm"
                    : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
                }`}
                disabled={isSubmitting}
              >
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-card text-brand-primary border border-stroke shadow-xs">
                  <Bot size={16} aria-hidden="true" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="font-semibold text-foreground flex items-center justify-between">
                    <span>Auto-assign</span>
                    {agentID === "" && <Check size={14} className="text-brand-primary" />}
                  </div>
                  <div className="text-[10px] text-content-muted mt-0.5">Let the system select the best agent</div>
                </div>
              </button>

              {agents.map((agent) => {
                const isSelected = agentID === agent.id;
                const initials = agent.name.split(/\s+/).map(n => n[0]).join("").slice(0, 2).toUpperCase();
                const isBusy = ["busy", "assigned", "running"].includes(agent.status);
                const isOffline = agent.status === "offline";
                return (
                  <button
                    key={agent.id}
                    type="button"
                    onClick={() => setAgentID(agent.id)}
                    role="radio"
                    aria-checked={isSelected}
                    className={`flex w-full items-center gap-3.5 rounded-lg border p-3 text-left text-xs transition-all duration-200 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus ${
                      isSelected
                        ? "border-brand-primary/45 bg-brand-primary-muted text-foreground font-semibold shadow-sm"
                        : "border-stroke bg-surface text-content-muted hover:border-stroke-focus hover:text-foreground hover:bg-surface/50"
                    }`}
                    disabled={isSubmitting}
                  >
                    <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-card font-mono font-bold text-foreground border border-stroke shadow-xs text-xs">
                      {initials}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="font-semibold text-foreground flex items-center justify-between">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="truncate">{agent.name}</span>
                          <span className={`inline-flex items-center gap-1 shrink-0 rounded-full px-1.5 py-0.5 text-[8px] font-bold uppercase tracking-wide border ${
                            isOffline
                              ? "bg-slate-500/10 text-slate-400 border-slate-500/20"
                              : isBusy
                              ? "bg-amber-500/10 text-amber-500 border-amber-500/20"
                              : "bg-emerald-500/10 text-emerald-500 border-emerald-500/20"
                          }`}>
                            <span className={`h-1.5 w-1.5 rounded-full ${
                              isOffline ? "bg-slate-400" : isBusy ? "bg-amber-500 animate-pulse" : "bg-emerald-500"
                            }`} />
                            {isOffline ? "Offline" : isBusy ? "Working" : "Free"}
                          </span>
                        </div>
                        {isSelected && <Check size={14} className="text-brand-primary shrink-0" />}
                      </div>
                      <div className="truncate text-[10px] text-content-muted mt-0.5">Role: {agent.role} • {agent.model_level_group || "Default Model Level Group"}</div>
                    </div>
                  </button>
                );
              })}
            </div>
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
            className="inline-flex items-center gap-1.5 rounded-lg bg-brand-primary px-5 py-2.5 text-sm font-semibold text-slate-950 hover:opacity-90 hover:shadow-md transition-all duration-150 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed focus:outline-none focus:ring-2 focus:ring-brand-primary-muted"
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
        <div className="absolute inset-4 z-20 flex flex-col rounded-xl border border-stroke bg-card shadow-2xl overflow-hidden animate-modal-in">
          {/* Header */}
          <div className="flex items-center justify-between border-b border-stroke p-4 shrink-0 bg-card/95 backdrop-blur-sm">
            <div className="flex items-center gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-brand-primary-muted border border-brand-primary/20 text-brand-primary">
                <FileText size={16} />
              </div>
              <div>
                <h3 className="font-sans text-sm font-bold text-foreground">Rich Description Editor</h3>
                <p className="text-[10px] text-content-muted font-medium mt-0.5">Compose markdown description with live preview</p>
              </div>
            </div>
            
            <div className="flex items-center gap-3">
              {/* Template selector */}
              <div className="relative">
                <select
                  onChange={(e) => {
                    const templateVal = e.target.value;
                    if (templateVal === "feature") {
                      setDescription(
                        "## Objective\nBrief summary of the feature.\n\n## Acceptance Criteria\n- [ ] Item 1\n- [ ] Item 2\n\n## Affected Components\n- List files/modules to modify."
                      );
                    } else if (templateVal === "bug") {
                      setDescription(
                        "## Problem\nDetail what is wrong and what errors are displayed.\n\n## Expected Behavior\nWhat is the correct flow?\n\n## Steps to Reproduce\n1. Go to...\n2. Run..."
                      );
                    } else if (templateVal === "refactor") {
                      setDescription(
                        "## Goals\n- [ ] Clean up redundant code\n- [ ] Enhance performance/readability\n\n## Plan\nDetail the changes planned."
                      );
                    }
                    e.target.value = ""; // Reset
                  }}
                  className="rounded-lg border border-stroke bg-surface px-2.5 py-1.5 text-xs text-foreground cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus font-medium"
                >
                  <option value="">Insert Template...</option>
                  <option value="feature">Feature Spec Template</option>
                  <option value="bug">Bug Fix Template</option>
                  <option value="refactor">Refactoring Template</option>
                </select>
              </div>

              <button
                type="button"
                onClick={() => setIsDescExpanded(false)}
                className="inline-flex items-center gap-1.5 rounded-lg border border-stroke bg-surface px-3 py-1.5 text-xs font-semibold text-foreground hover:bg-surface-code hover:border-stroke-focus transition-all duration-150 cursor-pointer focus:outline-none focus:ring-1 focus:ring-stroke-focus"
              >
                <Minimize2 size={12} />
                Done
              </button>
            </div>
          </div>

          {/* Editor Grid */}
          <div className="flex-1 grid grid-cols-2 overflow-hidden bg-surface/5">
            {/* Left: Input Textarea */}
            <div className="flex flex-col border-r border-stroke overflow-hidden h-full">
              <div className="flex items-center gap-2 border-b border-stroke px-4 py-2 bg-surface/30 shrink-0">
                <span className="font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted/80">Markdown Source</span>
              </div>
              <textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="flex-1 w-full p-4 text-xs font-mono bg-transparent text-foreground placeholder-content-muted/40 outline-none resize-none overflow-y-auto scrollbar-thin leading-relaxed"
                placeholder="Write your markdown description here..."
              />
              <div className="border-t border-stroke px-4 py-2 bg-surface/30 shrink-0 flex items-center justify-between text-[10px] text-content-muted/80">
                <span>{description.length} characters</span>
                <span>{description.split(/\s+/).filter(Boolean).length} words</span>
              </div>
            </div>

            {/* Right: Markdown Live Preview */}
            <div className="flex flex-col overflow-hidden h-full">
              <div className="flex items-center gap-2 border-b border-stroke px-4 py-2 bg-surface/30 shrink-0">
                <span className="font-mono text-[9px] font-bold uppercase tracking-wider text-content-muted/80">Live Preview</span>
              </div>
              <div className="flex-1 p-4 overflow-y-auto scrollbar-thin text-xs text-foreground leading-relaxed bg-card/40">
                {description.trim() ? (
                  <div className="space-y-3 font-sans break-words">
                    <ReactMarkdown 
                      remarkPlugins={[remarkGfm]}
                      components={{
                        h1: ({node, ...props}) => <h1 className="text-base font-bold text-foreground border-b border-stroke pb-1 pt-2 first:mt-0" {...props} />,
                        h2: ({node, ...props}) => <h2 className="text-sm font-bold text-foreground border-b border-stroke/50 pb-0.5 pt-2 first:mt-0" {...props} />,
                        h3: ({node, ...props}) => <h3 className="text-xs font-bold text-foreground pt-1.5" {...props} />,
                        ul: ({node, ...props}) => <ul className="list-disc pl-4 space-y-1 my-1" {...props} />,
                        ol: ({node, ...props}) => <ol className="list-decimal pl-4 space-y-1 my-1" {...props} />,
                        li: ({node, ...props}) => <li className="text-content font-medium" {...props} />,
                        p: ({node, ...props}) => <p className="text-content leading-relaxed my-1.5 font-medium" {...props} />,
                        code: ({node, ...props}) => <code className="bg-surface border border-stroke rounded px-1 py-0.5 font-mono text-[11px]" {...props} />,
                        pre: ({node, ...props}) => <pre className="bg-surface border border-stroke rounded-lg p-2.5 font-mono text-[11px] overflow-x-auto my-2" {...props} />,
                        blockquote: ({node, ...props}) => <blockquote className="border-l-2 border-brand-primary/40 pl-3 text-content-muted italic my-1.5" {...props} />,
                        table: ({node, ...props}) => <table className="w-full border-collapse border border-stroke text-[11px] my-2" {...props} />,
                        th: ({node, ...props}) => <th className="border border-stroke bg-surface p-1.5 text-left font-bold" {...props} />,
                        td: ({node, ...props}) => <td className="border border-stroke p-1.5" {...props} />,
                        input: ({node, ...props}) => <input type="checkbox" className="mr-1.5 cursor-pointer accent-brand-primary" disabled checked={props.checked} />
                      }}
                    >
                      {description}
                    </ReactMarkdown>
                  </div>
                ) : (
                  <div className="flex h-full items-center justify-center text-content-muted/50 italic text-[11px]">
                    Preview will render here as you type...
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({ label, action, children }: { label: string; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center justify-between">
        <div className="font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted/80">{label}</div>
        {action}
      </div>
      {children}
    </div>
  );
}
