"use client";

import { useState, useCallback, useEffect } from "react";
import { Edit2, Check, X } from "lucide-react";
import { useTaskDetail } from "./TaskDetailContext";
import { Markdown } from "@/components/ui/markdown";

export function DescriptionBody() {
  const {
    task,
    descriptionParts,
    updateTask,
  } = useTaskDetail();

  const [isEditingDesc, setIsEditingDesc] = useState(false);
  const [editedDesc, setEditedDesc] = useState(task?.description ?? "");
  const [isSaving, setIsSaving] = useState(false);
  const [isExpanded, setIsExpanded] = useState(false);

  useEffect(() => {
    if (task?.description) {
      setEditedDesc(task.description);
    }
  }, [task?.description]);

  const handleStartEditDesc = useCallback(() => {
    setEditedDesc(task?.description ?? "");
    setIsEditingDesc(true);
  }, [task?.description]);

  const handleSaveDesc = useCallback(async () => {
    setIsSaving(true);
    await updateTask({ description: editedDesc.trim() });
    setIsEditingDesc(false);
    setIsSaving(false);
  }, [editedDesc, updateTask]);

  const handleCancelDesc = useCallback(() => {
    setIsEditingDesc(false);
  }, []);

  if (!task) return null;

  return (
    <div className="space-y-4">
      {isEditingDesc ? (
        <div className="flex flex-col gap-2 max-w-4xl text-left">
          <textarea
            value={editedDesc}
            onChange={(e) => setEditedDesc(e.target.value)}
            className="text-sm text-foreground bg-surface border border-stroke rounded px-3 py-2 focus:outline-none focus:border-brand-primary min-h-[120px] resize-y w-full"
            disabled={isSaving}
            autoFocus
            placeholder="Detail the target objective, files to modify, or technical requirements."
          />
          <div className="flex gap-2 justify-end">
            <button
              onClick={handleCancelDesc}
              disabled={isSaving}
              className="px-3 py-1.5 text-xs font-semibold border border-stroke hover:bg-surface rounded transition cursor-pointer disabled:opacity-50 bg-transparent text-foreground"
            >
              Cancel
            </button>
            <button
              onClick={handleSaveDesc}
              disabled={isSaving}
              className="px-3 py-1.5 text-xs font-semibold bg-brand-primary text-slate-950 hover:opacity-90 rounded transition cursor-pointer disabled:opacity-50"
            >
              {isSaving ? "Saving..." : "Save Description"}
            </button>
          </div>
        </div>
      ) : (
        <div className="group relative flex items-start gap-2 text-left">
          <div className="min-w-0 flex-1 rounded-lg border border-stroke/50 bg-surface/20 p-4">
            {descriptionParts.body ? (
              <div className="flex flex-col items-start">
                <div className={`prose prose-sm max-w-none text-content-muted dark:prose-invert prose-headings:text-foreground prose-strong:text-foreground prose-p:leading-relaxed prose-li:leading-relaxed relative overflow-hidden transition-all duration-300 ${!isExpanded ? 'max-h-24' : ''}`}>
                  <Markdown content={descriptionParts.body} />
                  {!isExpanded && (
                    <div className="absolute bottom-0 left-0 right-0 h-12 bg-gradient-to-t from-surface/20 to-transparent pointer-events-none" />
                  )}
                </div>
                <button
                  onClick={() => setIsExpanded(!isExpanded)}
                  className="text-brand-primary text-xs font-medium hover:underline mt-2 cursor-pointer transition-colors"
                >
                  {isExpanded ? "Show less" : "Show more"}
                </button>
              </div>
            ) : (
              <p className="text-sm text-content-muted/70 italic">No description provided. Click the edit icon to add one.</p>
            )}
          </div>
          <button
            onClick={handleStartEditDesc}
            className="opacity-40 hover:opacity-100 focus:opacity-100 group-hover:opacity-100 focus-within:opacity-100 p-1.5 text-content-muted hover:text-foreground hover:bg-surface rounded transition shrink-0 cursor-pointer border border-transparent hover:border-stroke/30"
            title="Edit Description"
          >
            <Edit2 size={14} />
          </button>
        </div>
      )}

      {descriptionParts.context && (
        <div className="rounded-lg border border-warning/20 bg-warning/5 p-4 text-xs text-content-muted text-left">
          <div className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-warning">
            Request history (Legacy)
          </div>
          <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert prose-headings:text-foreground prose-strong:text-foreground prose-p:leading-relaxed prose-li:leading-relaxed">
            <Markdown content={descriptionParts.context} />
          </div>
        </div>
      )}

      {task.clarifications && task.clarifications.length > 0 && (
        <div className="rounded-lg border border-stroke/50 bg-surface/20 p-4 text-xs text-content-muted text-left">
          <div className="mb-2 font-mono text-[10px] font-bold uppercase tracking-wider text-warning">
            Clarification History
          </div>
          <div className="space-y-3">
            {task.clarifications.map((round) => (
              <div key={round.round} className="border-t border-stroke/30 pt-2.5 first:border-0 first:pt-0">
                <div className="text-[10px] font-semibold text-content-muted mb-1.5 flex justify-between items-center">
                  <span>Round {round.round}</span>
                  <span className="opacity-70">{new Date(round.timestamp).toLocaleString()}</span>
                </div>
                <div className="pl-3 border-l-2 border-warning/30 space-y-1.5 text-xs text-foreground/90 bg-warning/[0.02] p-2 rounded-r">
                  <div className="prose prose-sm max-w-none text-content-muted dark:prose-invert">
                    <Markdown content={round.response} />
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
