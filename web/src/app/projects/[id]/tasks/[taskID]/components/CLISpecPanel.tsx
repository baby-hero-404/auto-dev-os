"use client";

import { useState, useEffect } from "react";
import { FileText, ChevronDown, ChevronUp, Maximize2, Minimize2, Edit2, Save, X } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { tasks as tasksApi } from "@/lib/api/projects";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { mutate } from "swr";
import { useTaskDetail } from "./TaskDetailContext";

type SpecTab = "proposal" | "specs" | "design" | "tasks";

export function CLISpecPanel() {
  const { taskID, token } = useTaskDetail();
  const [tab, setTab] = useState<SpecTab>("proposal");
  const [isOpen, setIsOpen] = useState(true);
  const [isFullscreen, setIsFullscreen] = useState(false);
  
  const [isEditing, setIsEditing] = useState(false);
  const [editContent, setEditContent] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  // Prevent background scrolling when in fullscreen
  useEffect(() => {
    if (isFullscreen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = 'unset';
    }
    return () => { document.body.style.overflow = 'unset'; };
  }, [isFullscreen]);

  const { data: spec, error } = useAuthedSWR(
    ["task-spec", taskID],
    (token) => tasksApi.getSpec(taskID, token),
  );

  if (error || !spec) {
    return null;
  }

  const handleEdit = () => {
    const currentTabContent = tabs.find((t) => t.id === tab)?.content || "";
    setEditContent(currentTabContent);
    setIsEditing(true);
  };

  const handleSave = async () => {
    setIsSaving(true);
    try {
      await tasksApi.updateSpec(taskID, token, { [tab]: editContent });
      await mutate(["task-spec", taskID]);
      setIsEditing(false);
    } catch (e) {
      console.error("Failed to save spec", e);
    } finally {
      setIsSaving(false);
    }
  };

  const isEmptySpec = !spec.proposal && !spec.specs && !spec.design && !spec.tasks;

  if (isEmptySpec) {
    return (
      <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-8 shadow-lg flex flex-col items-center justify-center text-center gap-3">
        <FileText size={32} className="text-content-muted/50" />
        <div>
          <h2 className="font-heading text-base font-bold text-foreground">No Specification Data</h2>
          <p className="text-sm text-content-muted mt-1">There is no specification data available for this task yet.</p>
        </div>
      </div>
    );
  }

  const progressPct = spec.progress.total > 0 ? Math.round((spec.progress.done / spec.progress.total) * 100) : 0;

  const tabs: { id: SpecTab; label: string; content: string }[] = [
    { id: "proposal", label: "Proposal", content: spec.proposal },
    { id: "specs", label: "Specs", content: spec.specs },
    { id: "design", label: "Design", content: spec.design },
    { id: "tasks", label: "Tasks", content: spec.tasks },
  ];

  const containerClasses = isFullscreen
    ? "fixed inset-0 z-50 bg-background/95 backdrop-blur-xl p-6 md:p-12 flex flex-col overflow-hidden"
    : "relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-5 shadow-lg";

  return (
    <div className={containerClasses}>
      <div className={`flex flex-wrap items-center justify-between gap-4 ${isOpen || isFullscreen ? "mb-4 border-b border-stroke/40 pb-3" : ""}`}>
        <div className="flex items-center gap-2">
          {!isFullscreen && (
            <button
              type="button"
              onClick={() => setIsOpen((v) => !v)}
              className="flex items-center cursor-pointer text-left mr-1"
              aria-expanded={isOpen}
            >
              {isOpen ? <ChevronUp size={18} className="text-content-muted hover:text-foreground" /> : <ChevronDown size={18} className="text-content-muted hover:text-foreground" />}
            </button>
          )}
          <FileText size={18} className="text-brand-primary" />
          <h2 className="font-heading text-base font-bold text-foreground">OpenSpec (CLI Flow)</h2>
        </div>

        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2 bg-surface/50 px-3 py-1.5 rounded-full border border-stroke/50">
            <div className="h-1.5 w-24 rounded-full bg-surface overflow-hidden">
              <div className="h-full bg-brand-primary transition-all" style={{ width: `${progressPct}%` }} />
            </div>
            <span className="text-[10px] font-mono font-semibold text-content-muted">
              {spec.progress.done}/{spec.progress.total}
            </span>
          </div>
          
          <button
            type="button"
            onClick={() => {
              setIsFullscreen(!isFullscreen);
              if (!isFullscreen && !isOpen) setIsOpen(true);
            }}
            className="p-1.5 rounded-md text-content-muted hover:text-foreground hover:bg-surface border border-transparent hover:border-stroke transition-colors cursor-pointer"
            title={isFullscreen ? "Exit fullscreen" : "View fullscreen"}
          >
            {isFullscreen ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
          </button>
        </div>
      </div>

      {(isOpen || isFullscreen) && (
        <div className={`flex flex-col ${isFullscreen ? "flex-1 min-h-0" : ""}`}>
          <div className="flex gap-1.5 bg-surface/60 p-1.5 rounded-lg border border-stroke shadow-inner overflow-x-auto hide-scrollbar mb-4 shrink-0">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`px-4 py-2 rounded-md text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap ${
                  tab === t.id ? "bg-card text-brand-primary shadow-sm ring-1 ring-stroke" : "text-content-muted hover:text-foreground hover:bg-card/50"
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          <div className={`rounded-lg border border-stroke bg-card overflow-hidden shadow-inner text-sm flex flex-col ${
            isFullscreen ? "flex-1 min-h-0" : "max-h-[75vh] min-h-[40vh]"
          }`}>
            {isEditing ? (
              <div className="flex flex-col h-full flex-1 min-h-[40vh]">
                <div className="bg-surface/50 border-b border-stroke p-2 flex justify-between items-center">
                  <span className="text-xs font-semibold text-content-muted">Editing {tabs.find((t) => t.id === tab)?.label}</span>
                  <div className="flex gap-2">
                    <button onClick={() => setIsEditing(false)} className="px-3 py-1 text-xs rounded-md border border-stroke hover:bg-surface cursor-pointer flex items-center gap-1">
                      <X size={14} /> Cancel
                    </button>
                    <button onClick={() => handleSave()} disabled={isSaving} className="px-3 py-1 text-xs rounded-md bg-brand-primary text-white hover:bg-brand-primary/90 cursor-pointer flex items-center gap-1 disabled:opacity-50">
                      <Save size={14} /> {isSaving ? "Saving..." : "Save"}
                    </button>
                  </div>
                </div>
                <textarea
                  className="flex-1 w-full p-4 bg-transparent outline-none resize-none font-mono text-sm"
                  value={editContent}
                  onChange={(e) => setEditContent(e.target.value)}
                />
              </div>
            ) : (
              <div className="p-6 md:p-8 overflow-auto h-full relative group">
                <button 
                  onClick={handleEdit}
                  className="absolute top-4 right-4 p-2 bg-surface border border-stroke rounded-md text-content-muted hover:text-foreground hover:bg-card/80 opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer shadow-sm"
                  title="Edit this tab"
                >
                  <Edit2 size={16} />
                </button>
                <Markdown content={tabs.find((t) => t.id === tab)?.content || ""} />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
