"use client";

import { useState, useMemo } from "react";
import { FileText, AlertTriangle, Lightbulb, ShieldAlert, Code2, Layers } from "lucide-react";
import { Markdown } from "@/components/ui/markdown";
import { tasks as tasksApi } from "@/lib/api/projects";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { useTaskDetail } from "../TaskDetailContext";

type SpecTab = "proposal" | "specs" | "design" | "tasks";

interface ParsedSection {
  title: string;
  level: number;
  content: string;
}

// Simple parser to extract heading-based sections from markdown
function parseMarkdownSections(markdown: string): ParsedSection[] {
  if (!markdown) return [];
  const lines = markdown.split("\n");
  const sections: ParsedSection[] = [];
  let currentTitle = "Summary";
  let currentLevel = 1;
  let currentContent = "";

  for (const line of lines) {
    const match = line.match(/^(#{1,6})\s+(.*)/);
    if (match) {
      if (currentContent.trim() || currentTitle !== "Summary") {
        sections.push({ title: currentTitle, level: currentLevel, content: currentContent.trim() });
      }
      currentLevel = match[1].length;
      currentTitle = match[2].trim();
      currentContent = "";
    } else {
      currentContent += line + "\n";
    }
  }
  
  if (currentContent.trim() || sections.length === 0) {
    sections.push({ title: currentTitle, level: currentLevel, content: currentContent.trim() });
  }

  return sections;
}

function getIconForSection(title: string) {
  const t = title.toLowerCase();
  if (t.includes("problem") || t.includes("context") || t.includes("issue")) return <AlertTriangle size={18} className="text-amber-500" />;
  if (t.includes("root cause") || t.includes("cause")) return <Code2 size={18} className="text-rose-500" />;
  if (t.includes("fix") || t.includes("solution") || t.includes("change")) return <Lightbulb size={18} className="text-emerald-500" />;
  if (t.includes("risk")) return <ShieldAlert size={18} className="text-red-500" />;
  return <Layers size={18} className="text-brand-primary" />;
}

export function SpecViewer() {
  const { taskID, token } = useTaskDetail();
  const [tab, setTab] = useState<SpecTab>("proposal");

  const { data: spec, error } = useAuthedSWR(
    taskID ? ["task-spec", taskID] : null,
    (token) => tasksApi.getSpec(taskID, token),
  );

  const proposalSections = useMemo(() => {
    if (!spec?.proposal) return [];
    return parseMarkdownSections(spec.proposal);
  }, [spec?.proposal]);

  if (error || !spec) {
    return null;
  }

  const isEmptySpec = !spec.proposal && !spec.specs && !spec.design && !spec.tasks;

  if (isEmptySpec) {
    return (
      <div className="relative overflow-hidden rounded-xl border border-stroke/50 bg-card/60 backdrop-blur-xl p-8 shadow-sm flex flex-col items-center justify-center text-center gap-3">
        <FileText size={32} className="text-content-muted/50" />
        <div>
          <h2 className="font-heading text-base font-bold text-foreground">No Specification Data</h2>
          <p className="text-sm text-content-muted mt-1">There is no specification data available for this task yet.</p>
        </div>
      </div>
    );
  }

  const tabs: { id: SpecTab; label: string; content: string }[] = [
    { id: "proposal", label: "Proposal", content: spec.proposal },
    { id: "specs", label: "Specs", content: spec.specs },
    { id: "design", label: "Design", content: spec.design },
    { id: "tasks", label: "Tasks", content: spec.tasks },
  ];

  return (
    <div className="relative overflow-hidden rounded-xl bg-transparent flex flex-col h-full">
      <div className="flex flex-col flex-1 min-h-0">
        <div className="flex gap-2 border-b border-stroke/20 mb-4 shrink-0 pb-1">
          {tabs.map((t) => (
            <button
              key={t.id}
              onClick={() => setTab(t.id)}
              className={`px-3 py-2 text-xs font-bold uppercase tracking-wider transition-all duration-200 cursor-pointer whitespace-nowrap border-b-2 ${
                tab === t.id ? "border-brand-primary text-brand-primary" : "border-transparent text-content-muted hover:text-foreground hover:border-stroke"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        <div className="overflow-auto max-h-[calc(100vh-320px)] hide-scrollbar pb-6 pr-2">
          {tab === "proposal" ? (
            <div className="flex flex-col gap-4">
              {proposalSections.map((section, idx) => (
                <div key={idx} className="rounded-xl border border-stroke/15 bg-card/40 backdrop-blur-sm p-5 shadow-sm hover:shadow-md transition-shadow">
                  <div className="flex items-center gap-2.5 mb-3 pb-2 border-b border-stroke/10">
                    {getIconForSection(section.title)}
                    <h3 className="text-base font-bold text-foreground m-0">{section.title}</h3>
                  </div>
                  <div className="text-sm prose prose-sm dark:prose-invert max-w-none">
                    <Markdown content={section.content} />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="rounded-xl border border-stroke/15 bg-card/40 p-6 shadow-sm text-sm">
              <Markdown content={tabs.find((t) => t.id === tab)?.content || ""} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
