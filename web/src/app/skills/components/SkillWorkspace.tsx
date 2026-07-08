import { Check, ChevronRight, Copy, FileCode2, FileText, Folder, Terminal, Zap } from "lucide-react";
import type { Skill } from "@/lib/types";
import { Markdown } from "@/components/ui/markdown";
import { formatSize, parseSkillMeta } from "../utils";
import { Detail, EmptyState, LoadingState, PanelHeader, Tag } from "./CommonUI";

export type SourceFile = {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
};

interface SkillWorkspaceProps {
  selectedSkill: Skill | null;
  currentFolderPath: string;
  files: SourceFile[];
  selectedFilePath: string;
  fileContent: string;
  isLoadingFiles: boolean;
  isLoadingContent: boolean;
  copiedSkillId: string | null;
  copiedContent: boolean;
  sourceViewMode: "preview" | "raw";
  onCopySkill: (skill: Skill) => void;
  onCopyContent: () => void;
  onFolderChange: (path: string) => void;
  onFileSelect: (path: string) => void;
  onSourceViewModeChange: (mode: "preview" | "raw") => void;
}

export function SkillWorkspace({
  selectedSkill,
  currentFolderPath,
  files,
  selectedFilePath,
  fileContent,
  isLoadingFiles,
  isLoadingContent,
  copiedSkillId,
  copiedContent,
  sourceViewMode,
  onCopySkill,
  onCopyContent,
  onFolderChange,
  onFileSelect,
  onSourceViewModeChange,
}: SkillWorkspaceProps) {
  if (!selectedSkill) {
    return (
      <section className="rounded-lg border border-dashed border-stroke bg-panel/30 p-8 xl:col-span-2">
        <EmptyState
          icon={Zap}
          title="No skill selected"
          description="Select a skill from the catalog. The first matching skill auto-loads after sync."
        />
      </section>
    );
  }

  const meta = parseSkillMeta(selectedSkill);

  return (
    <>
      <section className="flex max-h-[calc(100vh-220px)] min-h-[620px] flex-col gap-5">
        <div className="rounded-lg border border-stroke bg-card p-4">
          <div className="flex items-start justify-between gap-4">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h3 className="truncate font-mono text-lg font-semibold text-foreground">{selectedSkill.name}</h3>
                <Tag>{meta.category}</Tag>
              </div>
              <p className="mt-1 text-sm leading-relaxed text-content-muted">
                {selectedSkill.description || "No description loaded."}
              </p>
            </div>
            <button
              type="button"
              onClick={() => onCopySkill(selectedSkill)}
              className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-stroke px-2.5 py-1.5 text-xs font-semibold text-foreground transition hover:bg-surface cursor-pointer"
              title="Copy skill reference"
            >
              {copiedSkillId === selectedSkill.id ? <Check size={14} className="text-success" /> : <Copy size={14} />}
              Reference
            </button>
          </div>

          <div className="mt-4 grid gap-2">
            <Detail label="Registry" value={meta.registry} />
            <Detail label="Repository" value={meta.repo} />
            <Detail label="Source path" value={meta.path || "Not mapped"} />
          </div>
        </div>

        <FileExplorer
          currentFolderPath={currentFolderPath}
          files={files}
          selectedFilePath={selectedFilePath}
          isLoading={isLoadingFiles}
          onFolderChange={onFolderChange}
          onFileSelect={onFileSelect}
        />
      </section>

      <SourceViewer
        selectedFilePath={selectedFilePath}
        fileContent={fileContent}
        isLoading={isLoadingContent}
        copiedContent={copiedContent}
        viewMode={sourceViewMode}
        onViewModeChange={onSourceViewModeChange}
        onCopyContent={onCopyContent}
      />
    </>
  );
}

interface FileExplorerProps {
  currentFolderPath: string;
  files: SourceFile[];
  selectedFilePath: string;
  isLoading: boolean;
  onFolderChange: (path: string) => void;
  onFileSelect: (path: string) => void;
}

export function FileExplorer({
  currentFolderPath,
  files,
  selectedFilePath,
  isLoading,
  onFolderChange,
  onFileSelect,
}: FileExplorerProps) {
  const parts = currentFolderPath.split("/").filter(Boolean);
  const parentPath = parts.slice(0, -1).join("/");

  return (
    <div className="rounded-lg border border-stroke bg-card p-4">
      <PanelHeader icon={Folder} title="Explorer" detail={currentFolderPath || "repo root"} />

      {parts.length > 0 && (
        <div className="mb-3 flex min-w-0 flex-wrap items-center gap-1 rounded-md border border-stroke bg-surface/35 px-2 py-1.5 text-xs">
          <button type="button" onClick={() => onFolderChange("")} className="font-mono text-brand-primary hover:underline cursor-pointer">
            root
          </button>
          {parts.map((part, index) => {
            const path = parts.slice(0, index + 1).join("/");
            return (
              <span key={path} className="flex min-w-0 items-center gap-1">
                <ChevronRight size={12} className="text-content-muted" />
                <button
                  type="button"
                  onClick={() => onFolderChange(path)}
                  className="max-w-28 truncate font-mono text-content-muted hover:text-foreground cursor-pointer"
                  title={part}
                >
                  {part}
                </button>
              </span>
            );
          })}
        </div>
      )}

      <div className="max-h-[460px] space-y-1 overflow-y-auto pr-1">
        {isLoading ? (
          <LoadingState label="Loading folder" />
        ) : (
          <>
            {currentFolderPath && (
              <button
                type="button"
                onClick={() => onFolderChange(parentPath)}
                className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm font-semibold text-brand-primary transition hover:bg-surface cursor-pointer"
              >
                <Folder size={15} />
                Back
              </button>
            )}

            {files.map((file) => {
              const isSelected = selectedFilePath === file.path;
              return (
                <button
                  key={file.path}
                  type="button"
                  onClick={() => (file.is_dir ? onFolderChange(file.path) : onFileSelect(file.path))}
                  className={`flex w-full items-center justify-between gap-3 rounded-md px-2.5 py-2 text-left text-sm transition cursor-pointer ${
                    isSelected ? "bg-brand-primary/15 text-brand-primary" : "text-foreground hover:bg-surface"
                  }`}
                >
                  <span className="flex min-w-0 items-center gap-2">
                    {file.is_dir ? (
                      <Folder size={15} className="shrink-0 text-warning" />
                    ) : (
                      <FileCode2 size={15} className="shrink-0 text-brand-primary" />
                    )}
                    <span className="truncate font-mono text-xs">{file.name}</span>
                  </span>
                  {!file.is_dir && <span className="shrink-0 font-mono text-[10px] text-content-muted">{formatSize(file.size)}</span>}
                </button>
              );
            })}

            {files.length === 0 && (
              <div className="rounded-md border border-dashed border-stroke p-6 text-center text-xs text-content-muted">
                This directory is empty.
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

interface SourceViewerProps {
  selectedFilePath: string;
  fileContent: string;
  isLoading: boolean;
  copiedContent: boolean;
  viewMode: "preview" | "raw";
  onViewModeChange: (mode: "preview" | "raw") => void;
  onCopyContent: () => void;
}

export function SourceViewer({
  selectedFilePath,
  fileContent,
  isLoading,
  copiedContent,
  viewMode,
  onViewModeChange,
  onCopyContent,
}: SourceViewerProps) {
  const isMarkdown = selectedFilePath.toLowerCase().endsWith(".md");
  const shouldPreview = isMarkdown && viewMode === "preview";

  return (
    <section className="flex max-h-[calc(100vh-220px)] min-h-[620px] min-w-0 flex-col rounded-lg border border-stroke bg-card p-4">
      <div className="mb-3 flex items-center justify-between gap-3 border-b border-stroke pb-3">
        <div className="flex items-center gap-2.5 min-w-0">
          <PanelHeader icon={Terminal} title="Source" detail={selectedFilePath || "No file selected"} compact />
          <span className="rounded-full border border-stroke bg-surface/60 px-2 py-0.5 text-[9px] font-semibold uppercase tracking-wide text-content-muted">
            Read Only
          </span>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          {isMarkdown && (
            <div className="flex rounded-md border border-stroke bg-background p-0.5">
              <button
                type="button"
                onClick={() => onViewModeChange("preview")}
                className={`rounded px-2 py-1 text-xs font-semibold transition cursor-pointer ${
                  viewMode === "preview" ? "bg-surface text-foreground" : "text-content-muted hover:text-foreground"
                }`}
              >
                Preview
              </button>
              <button
                type="button"
                onClick={() => onViewModeChange("raw")}
                className={`rounded px-2 py-1 text-xs font-semibold transition cursor-pointer ${
                  viewMode === "raw" ? "bg-surface text-foreground" : "text-content-muted hover:text-foreground"
                }`}
              >
                Raw
              </button>
            </div>
          )}
          <button
            type="button"
            onClick={onCopyContent}
            disabled={!fileContent}
            className="inline-flex items-center gap-1.5 rounded-md border border-stroke px-2.5 py-1.5 text-xs font-semibold text-foreground transition hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer"
          >
            {copiedContent ? <Check size={14} className="text-success" /> : <Copy size={14} />}
            Copy
          </button>
        </div>
      </div>

      {selectedFilePath ? (
        <div className="min-h-0 flex-1 overflow-y-auto overflow-x-auto overscroll-contain rounded-md border border-stroke bg-background p-4 scrollbar-thin">
          {isLoading ? (
            <LoadingState label="Loading file" />
          ) : shouldPreview ? (
            <div className="font-sans text-sm text-content-muted leading-relaxed">
              <Markdown content={fileContent || "(Empty file)"} />
            </div>
          ) : (
            <pre className="whitespace-pre-wrap font-mono text-xs leading-relaxed text-foreground">{fileContent || "(Empty file)"}</pre>
          )}
        </div>
      ) : (
        <EmptyState icon={FileText} title="No file selected" description="Select a file from the explorer to inspect its content." />
      )}
    </section>
  );
}
