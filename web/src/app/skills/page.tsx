"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  Check,
  ChevronRight,
  Copy,
  Database,
  FileCode2,
  FileText,
  Folder,
  GitBranch,
  Loader2,
  RefreshCw,
  Search,
  Terminal,
  Trash2,
  Zap,
  type LucideIcon,
} from "lucide-react";
import { toast, Toaster } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Skill, SkillSource } from "@/lib/types";
import { Markdown } from "@/components/ui/markdown";

const DEFAULT_SOURCE_URL = "https://github.com/baby-hero-404/prompt_base.git";

type SourceFile = {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
};

type SkillMeta = {
  repo: string;
  category: string;
  registry: string;
  path: string;
  source: string;
};

function parseSkillMeta(skill: Skill): SkillMeta {
  const fallback = {
    repo: "unknown",
    category: "general",
    registry: skill.id,
    path: "",
    source: "registry",
  };

  try {
    const parsed = typeof skill.schema === "string" ? JSON.parse(skill.schema) : skill.schema;
    return {
      repo: typeof parsed?.repo === "string" ? parsed.repo : fallback.repo,
      category: typeof parsed?.category === "string" ? parsed.category : fallback.category,
      registry: typeof parsed?.registry === "string" ? parsed.registry : fallback.registry,
      path: typeof parsed?.path === "string" ? parsed.path : fallback.path,
      source: typeof parsed?.source === "string" ? parsed.source : fallback.source,
    };
  } catch {
    return fallback;
  }
}

function repoNameFromURL(gitURL: string) {
  const cleaned = gitURL.trim().replace(/\.git$/, "");
  const parts = cleaned.split("/").filter(Boolean);
  return parts.at(-1) || "unknown";
}

function cleanRepoPath(path: string) {
  const parts = path.split("/").filter(Boolean);
  if (parts[0] === "git" && parts.length > 1) {
    return parts.slice(2).join("/");
  }
  return path;
}

function folderForSkill(skill: Skill) {
  const path = cleanRepoPath(parseSkillMeta(skill).path);
  if (!path) return "";
  if (path.endsWith(".md")) {
    const parts = path.split("/").filter(Boolean);
    if (parts.length <= 1) return "";
    return parts.slice(0, -1).join("/");
  }
  return path;
}

function fileForSkill(skill: Skill) {
  const path = cleanRepoPath(parseSkillMeta(skill).path);
  if (!path) return "SKILL.md";
  if (path.endsWith(".md")) return path;
  return `${path}/SKILL.md`;
}

function formatDateTime(value?: string) {
  if (!value) return "Never";
  return new Intl.DateTimeFormat("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function formatSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 KB";
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KB`;
}

export default function SkillsPage() {
  const session = useSession();
  const token = session?.token ?? "";

  const [newSourceURL, setNewSourceURL] = useState(DEFAULT_SOURCE_URL);
  const [isAddingSource, setIsAddingSource] = useState(false);
  const [syncingSourceID, setSyncingSourceID] = useState("");
  const [deletingSourceID, setDeletingSourceID] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedSkill, setSelectedSkill] = useState<Skill | null>(null);
  const [currentFolderPath, setCurrentFolderPath] = useState("");
  const [filesList, setFilesList] = useState<SourceFile[]>([]);
  const [isLoadingFiles, setIsLoadingFiles] = useState(false);
  const [selectedFilePath, setSelectedFilePath] = useState("");
  const [fileContent, setFileContent] = useState("");
  const [isLoadingContent, setIsLoadingContent] = useState(false);
  const [copiedSkillId, setCopiedSkillId] = useState<string | null>(null);
  const [copiedContent, setCopiedContent] = useState(false);
  const [selectedCategory, setSelectedCategory] = useState("all");
  const [sourceViewMode, setSourceViewMode] = useState<"preview" | "raw">("preview");

  const { data: rawSkills, mutate: mutateSkills, isLoading: isLoadingSkills } = useAuthedSWR<Skill[]>(
    token ? ["global-skills"] : null,
    (t) => api.listSkills(t),
  );
  const skills = rawSkills || [];

  const { data: rawSources, mutate: mutateSources, isLoading: isLoadingSources } = useAuthedSWR<SkillSource[]>(
    token ? ["skill-sources"] : null,
    (t) => api.listSkillSources(t),
  );
  const sources = rawSources || [];

  useEffect(() => {
    if (sources.length === 0 && !newSourceURL.trim()) {
      const timer = setTimeout(() => {
        setNewSourceURL(DEFAULT_SOURCE_URL);
      }, 0);
      return () => clearTimeout(timer);
    }
  }, [sources.length, newSourceURL]);

  const selectedSourceID = useMemo(() => {
    if (!selectedSkill) return "";
    const meta = parseSkillMeta(selectedSkill);
    const source = sources.find((item) => repoNameFromURL(item.url) === meta.repo);
    return source?.id ?? "";
  }, [selectedSkill, sources]);

  useEffect(() => {
    if (!token || !selectedSourceID) {
      const timer = setTimeout(() => {
        setFilesList([]);
      }, 0);
      return () => clearTimeout(timer);
    }

    let isActive = true;

    async function loadFiles() {
      setIsLoadingFiles(true);
      try {
        const files = await api.listSkillSourceFiles(selectedSourceID, currentFolderPath, token);
        const sorted = [...files].sort((a, b) => {
          if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
          return a.name.localeCompare(b.name);
        });
        if (isActive) setFilesList(sorted);
      } catch {
        if (isActive) setFilesList([]);
      } finally {
        if (isActive) setIsLoadingFiles(false);
      }
    }

    loadFiles();

    return () => {
      isActive = false;
    };
  }, [currentFolderPath, selectedSourceID, token]);

  useEffect(() => {
    if (!token || !selectedSourceID || !selectedFilePath) {
      const timer = setTimeout(() => {
        setFileContent("");
      }, 0);
      return () => clearTimeout(timer);
    }

    let isActive = true;

    async function loadContent() {
      setIsLoadingContent(true);
      try {
        const result = await api.getSkillSourceFileContent(selectedSourceID, selectedFilePath, token);
        if (isActive) setFileContent(result.content);
      } catch {
        if (isActive) setFileContent("");
      } finally {
        if (isActive) setIsLoadingContent(false);
      }
    }

    loadContent();

    return () => {
      isActive = false;
    };
  }, [selectedFilePath, selectedSourceID, token]);

  const sourceCount = sources.length;
  const syncedSourceCount = sources.filter((source) => source.status === "synced").length;
  const categoryCount = useMemo(() => new Set(skills.map((skill) => parseSkillMeta(skill).category)).size, [skills]);
  const categories = useMemo(() => {
    return Array.from(new Set(skills.map((skill) => parseSkillMeta(skill).category))).sort((a, b) => a.localeCompare(b));
  }, [skills]);
  const filteredSkills = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return skills.filter((skill) => {
      const meta = parseSkillMeta(skill);
      if (selectedCategory !== "all" && meta.category !== selectedCategory) return false;
      if (!query) return true;
      return [
        skill.name,
        skill.description || "",
        meta.category,
        meta.registry,
        meta.repo,
      ].some((value) => value.toLowerCase().includes(query));
    });
  }, [searchQuery, selectedCategory, skills]);

  function selectSkill(skill: Skill) {
    const folder = folderForSkill(skill);
    setSelectedSkill(skill);
    setCurrentFolderPath(folder);
    setSelectedFilePath(fileForSkill(skill));
    setFileContent("");
    setSourceViewMode("preview");
  }

  async function addSource(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token || !newSourceURL.trim()) return;

    if (sources.length >= 1) {
      toast.error("Only one skill repository can be connected at a time.");
      return;
    }

    setIsAddingSource(true);
    try {
      await api.createSkillSource(token, { url: newSourceURL.trim() });
      toast.success("Repository connected and synchronized.");
      setNewSourceURL("");
      await mutateSources();
      await mutateSkills();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to connect repository");
    } finally {
      setIsAddingSource(false);
    }
  }

  async function syncSource(sourceID: string) {
    if (!token) return;
    setSyncingSourceID(sourceID);
    try {
      await api.syncSkillSource(sourceID, token);
      toast.success("Repository sync completed.");
      await mutateSources();
      await mutateSkills();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Sync failed");
    } finally {
      setSyncingSourceID("");
    }
  }

  async function deleteSource(sourceID: string) {
    if (!token) return;
    setDeletingSourceID(sourceID);
    try {
      await api.deleteSkillSource(sourceID, token);
      toast.success("Repository source disconnected.");
      setSelectedSkill(null);
      setFilesList([]);
      setFileContent("");
      await mutateSources();
      await mutateSkills();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to disconnect source");
    } finally {
      setDeletingSourceID("");
    }
  }

  function copySkillReference(skill: Skill) {
    navigator.clipboard.writeText(skill.name);
    setCopiedSkillId(skill.id);
    toast.success(`Copied skill reference: ${skill.name}`);
    window.setTimeout(() => setCopiedSkillId(null), 1600);
  }

  function copyCurrentFile() {
    if (!fileContent) return;
    navigator.clipboard.writeText(fileContent);
    setCopiedContent(true);
    toast.success("Copied file content.");
    window.setTimeout(() => setCopiedContent(false), 1600);
  }

  return (
    <DashboardLayout>
      <Toaster closeButton position="top-right" richColors />

      <div className="mb-6 flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="font-mono text-2xl font-semibold text-foreground">Skills</h2>
          <p className="mt-1 max-w-2xl text-sm text-content-muted">
            Connect a skill repository, sync agent capabilities, and inspect source files before using them in workflows.
          </p>
        </div>
        <div className="grid grid-cols-3 gap-2 text-right text-xs">
          <Metric label="Skills" value={skills.length} />
          <Metric label="Categories" value={categoryCount} />
          <Metric label="Sources" value={`${syncedSourceCount}/${sourceCount}`} />
        </div>
      </div>

      {/* Top Banner Control for Repository Connection */}
      {isLoadingSources ? (
        <div className="mb-6 rounded-lg border border-stroke bg-card/60 p-4">
          <LoadingState label="Loading repository status..." />
        </div>
      ) : sources.length === 0 ? (
        <div className="mb-6 rounded-lg border border-stroke bg-card/60 backdrop-blur-sm p-4">
          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand-primary/10 text-brand-primary">
                <GitBranch size={18} />
              </div>
              <div>
                <h3 className="text-sm font-semibold text-foreground">Connect Skills Repository</h3>
                <p className="text-xs text-content-muted">Connect an external source to load agent capability modules.</p>
              </div>
            </div>
            <form onSubmit={addSource} className="flex min-w-0 max-w-xl flex-1 flex-row items-center gap-2">
              <input
                type="url"
                value={newSourceURL}
                onChange={(event) => setNewSourceURL(event.target.value)}
                placeholder="https://github.com/baby-hero-404/prompt_base.git"
                className="min-w-0 flex-1 rounded-md border border-stroke bg-background px-3 py-1.5 text-xs text-foreground outline-none transition focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20"
                required
              />
              <button
                type="submit"
                disabled={isAddingSource || !newSourceURL.trim()}
                className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-brand-primary px-3.5 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {isAddingSource ? <Loader2 size={13} className="animate-spin" /> : <GitBranch size={13} />}
                Connect
              </button>
              <button
                type="button"
                onClick={() => setNewSourceURL(DEFAULT_SOURCE_URL)}
                className="inline-flex shrink-0 items-center gap-1 rounded-md border border-stroke bg-surface/50 px-2.5 py-1.5 text-xs font-medium text-foreground transition hover:bg-surface"
              >
                Use Default
              </button>
            </form>
          </div>
        </div>
      ) : (
        <div className="mb-6 rounded-lg border border-stroke bg-card/60 backdrop-blur-sm p-4">
          {sources.map((source) => {
            const isSyncing = syncingSourceID === source.id;
            const isDeleting = deletingSourceID === source.id;
            return (
              <div key={source.id}>
                <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                  <div className="flex items-center gap-3 min-w-0">
                    <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-success/10 text-success">
                      <GitBranch size={18} />
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="truncate text-sm font-semibold text-foreground">{repoNameFromURL(source.url)}</span>
                        <StatusPill status={source.status} />
                      </div>
                      <div className="mt-0.5 truncate font-mono text-[11px] text-content-muted">{source.url}</div>
                    </div>
                  </div>
                  
                  <div className="flex flex-wrap items-center gap-3 text-xs">
                    <div className="flex items-center gap-1.5 rounded-md border border-stroke bg-surface/30 px-2.5 py-1">
                      <span className="text-[10px] uppercase tracking-wider text-content-muted">Last Synced:</span>
                      <span className="font-mono font-medium text-foreground">{formatDateTime(source.last_synced_at)}</span>
                    </div>
                    
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => syncSource(source.id)}
                        disabled={isSyncing || isDeleting}
                        className="inline-flex items-center gap-1.5 rounded-md border border-stroke bg-background px-3 py-1.5 font-semibold text-foreground transition hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {isSyncing ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
                        Sync
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteSource(source.id)}
                        disabled={isDeleting || isSyncing}
                        className="inline-flex items-center gap-1.5 rounded-md border border-danger/30 bg-background px-3 py-1.5 font-semibold text-danger transition hover:bg-danger/5 disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {isDeleting ? <Loader2 size={13} className="animate-spin" /> : <Trash2 size={13} />}
                        Disconnect
                      </button>
                    </div>
                  </div>
                </div>
                {source.error && (
                  <div className="mt-3 flex gap-2 rounded-md border border-danger/20 bg-danger/5 p-3 text-xs text-danger">
                    <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                    <span className="break-words">{source.error}</span>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <div className="grid min-h-[720px] items-start gap-5 xl:grid-cols-[360px_330px_minmax(0,1fr)]">
        <CatalogPanel
          skills={filteredSkills}
          selectedSkillID={selectedSkill?.id ?? ""}
          searchQuery={searchQuery}
          categories={categories}
          selectedCategory={selectedCategory}
          totalSkills={skills.length}
          isLoading={isLoadingSkills}
          hasSource={sources.length > 0}
          onSearchChange={setSearchQuery}
          onCategoryChange={setSelectedCategory}
          onSelectSkill={selectSkill}
        />

        <SkillWorkspace
          selectedSkill={selectedSkill}
          currentFolderPath={currentFolderPath}
          files={filesList}
          selectedFilePath={selectedFilePath}
          fileContent={fileContent}
          isLoadingFiles={isLoadingFiles}
          isLoadingContent={isLoadingContent}
          copiedSkillId={copiedSkillId}
          copiedContent={copiedContent}
          sourceViewMode={sourceViewMode}
          onCopySkill={copySkillReference}
          onCopyContent={copyCurrentFile}
          onFolderChange={setCurrentFolderPath}
          onFileSelect={setSelectedFilePath}
          onSourceViewModeChange={setSourceViewMode}
        />
      </div>
    </DashboardLayout>
  );
}

function Metric({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="min-w-20 rounded-md border border-stroke bg-panel px-3 py-2">
      <div className="font-mono text-sm font-semibold text-foreground">{value}</div>
      <div className="mt-0.5 text-[10px] uppercase tracking-wide text-content-muted">{label}</div>
    </div>
  );
}



function CatalogPanel({
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
}: {
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
}) {
  return (
    <section className="flex max-h-[calc(100vh-220px)] min-h-[620px] flex-col rounded-lg border border-stroke bg-card p-4">
      <PanelHeader icon={Zap} title="Catalog" detail={`${skills.length} of ${totalSkills} skills`} />

      <div className="relative mb-3">
        <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-content-muted" />
        <input
          value={searchQuery}
          onChange={(event) => onSearchChange(event.target.value)}
          placeholder="Search by name, category, or repo"
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
                className={`w-full rounded-md border p-3 text-left transition ${
                  isSelected
                    ? "border-brand-primary bg-brand-primary/10"
                    : "border-stroke bg-surface/25 hover:border-stroke-strong hover:bg-surface/50"
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <span className="min-w-0 truncate font-mono text-sm font-semibold text-foreground">{skill.name}</span>
                  <ChevronRight size={14} className="mt-0.5 shrink-0 text-content-muted" />
                </div>
                <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-content-muted">
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
          <EmptyState
            icon={Database}
            title={hasSource ? "No skills found" : "No repository connected"}
            description={hasSource ? "Try a different search term or sync the repository." : "Connect a repository to sync the catalog."}
          />
        )}
      </div>
    </section>
  );
}

function SkillWorkspace({
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
}: {
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
}) {
  if (!selectedSkill) {
    return (
      <section className="rounded-lg border border-dashed border-stroke bg-panel/30 p-8 xl:col-span-2">
        <EmptyState
          icon={Zap}
          title="Select a skill"
          description="Choose a catalog entry to inspect its metadata, source folder, and file contents."
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
              className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-stroke px-2.5 py-1.5 text-xs font-semibold text-foreground transition hover:bg-surface"
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

function FileExplorer({
  currentFolderPath,
  files,
  selectedFilePath,
  isLoading,
  onFolderChange,
  onFileSelect,
}: {
  currentFolderPath: string;
  files: SourceFile[];
  selectedFilePath: string;
  isLoading: boolean;
  onFolderChange: (path: string) => void;
  onFileSelect: (path: string) => void;
}) {
  const parts = currentFolderPath.split("/").filter(Boolean);
  const parentPath = parts.slice(0, -1).join("/");

  return (
    <div className="rounded-lg border border-stroke bg-card p-4">
      <PanelHeader icon={Folder} title="Explorer" detail={currentFolderPath || "repo root"} />

      {parts.length > 0 && (
        <div className="mb-3 flex min-w-0 flex-wrap items-center gap-1 rounded-md border border-stroke bg-surface/35 px-2 py-1.5 text-xs">
          <button type="button" onClick={() => onFolderChange("")} className="font-mono text-brand-primary hover:underline">
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
                  className="max-w-28 truncate font-mono text-content-muted hover:text-foreground"
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
                className="flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left text-sm font-semibold text-brand-primary transition hover:bg-surface"
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
                  className={`flex w-full items-center justify-between gap-3 rounded-md px-2.5 py-2 text-left text-sm transition ${
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

function SourceViewer({
  selectedFilePath,
  fileContent,
  isLoading,
  copiedContent,
  viewMode,
  onViewModeChange,
  onCopyContent,
}: {
  selectedFilePath: string;
  fileContent: string;
  isLoading: boolean;
  copiedContent: boolean;
  viewMode: "preview" | "raw";
  onViewModeChange: (mode: "preview" | "raw") => void;
  onCopyContent: () => void;
}) {
  const isMarkdown = selectedFilePath.toLowerCase().endsWith(".md");
  const shouldPreview = isMarkdown && viewMode === "preview";

  return (
    <section className="flex max-h-[calc(100vh-220px)] min-h-[620px] min-w-0 flex-col rounded-lg border border-stroke bg-card p-4">
      <div className="mb-3 flex items-center justify-between gap-3 border-b border-stroke pb-3">
        <PanelHeader icon={Terminal} title="Source" detail={selectedFilePath || "No file selected"} compact />
        <div className="flex shrink-0 items-center gap-2">
          {isMarkdown && (
            <div className="flex rounded-md border border-stroke bg-background p-0.5">
              <button
                type="button"
                onClick={() => onViewModeChange("preview")}
                className={`rounded px-2 py-1 text-xs font-semibold transition ${
                  viewMode === "preview" ? "bg-surface text-foreground" : "text-content-muted hover:text-foreground"
                }`}
              >
                Preview
              </button>
              <button
                type="button"
                onClick={() => onViewModeChange("raw")}
                className={`rounded px-2 py-1 text-xs font-semibold transition ${
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
            className="inline-flex items-center gap-1.5 rounded-md border border-stroke px-2.5 py-1.5 text-xs font-semibold text-foreground transition hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
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

function PanelHeader({
  icon: Icon,
  title,
  detail,
  compact = false,
}: {
  icon: LucideIcon;
  title: string;
  detail?: string;
  compact?: boolean;
}) {
  return (
    <div className={compact ? "min-w-0" : "mb-4"}>
      <div className="flex min-w-0 items-center gap-2">
        <Icon size={16} className="shrink-0 text-brand-primary" />
        <span className="truncate text-sm font-semibold text-foreground">{title}</span>
      </div>
      {detail && <div className="mt-0.5 truncate text-xs text-content-muted">{detail}</div>}
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  const normalized = status.toLowerCase();
  const color =
    normalized === "synced"
      ? "border-success/30 bg-success/10 text-success"
      : normalized === "syncing"
        ? "border-warning/30 bg-warning/10 text-warning"
        : "border-danger/30 bg-danger/10 text-danger";

  return (
    <span className={`rounded-full border px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide ${color}`}>
      {status || "unknown"}
    </span>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 rounded-md border border-stroke bg-surface/35 px-2.5 py-2">
      <div className="text-[10px] uppercase tracking-wide text-content-muted">{label}</div>
      <div className="mt-0.5 truncate font-mono text-xs font-semibold text-foreground" title={value}>
        {value}
      </div>
    </div>
  );
}

function Tag({ children }: { children: string }) {
  return (
    <span className="rounded border border-stroke bg-background px-1.5 py-0.5 font-mono text-[10px] font-semibold uppercase tracking-wide text-content-muted">
      {children}
    </span>
  );
}

function CategoryChip({
  active,
  children,
  onClick,
}: {
  active: boolean;
  children: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`whitespace-nowrap rounded-md border px-2.5 py-1 text-xs font-semibold transition ${
        active
          ? "border-brand-primary bg-brand-primary/10 text-brand-primary"
          : "border-stroke bg-background text-content-muted hover:bg-surface hover:text-foreground"
      }`}
    >
      {children}
    </button>
  );
}

function LoadingState({ label }: { label: string }) {
  return (
    <div className="flex min-h-28 flex-col items-center justify-center gap-2 text-content-muted">
      <Loader2 size={20} className="animate-spin text-brand-primary" />
      <span className="text-xs">{label}</span>
    </div>
  );
}

function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: LucideIcon;
  title: string;
  description: string;
}) {
  return (
    <div className="flex min-h-56 flex-col items-center justify-center rounded-md border border-dashed border-stroke p-6 text-center">
      <Icon size={28} className="text-content-muted/60" />
      <div className="mt-3 text-sm font-semibold text-foreground">{title}</div>
      <p className="mt-1 max-w-sm text-xs leading-relaxed text-content-muted">{description}</p>
    </div>
  );
}
