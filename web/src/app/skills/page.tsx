"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { toast, Toaster } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { api } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Skill, SkillSource } from "@/lib/types";

import {
  fileForSkill,
  folderForSkill,
  parseSkillMeta,
  repoNameFromURL,
  validateSkillRepoURL,
} from "./utils";
import { LoadingState } from "./components/CommonUI";
import { SkillsGuide } from "./components/SkillsGuide";
import { CatalogPanel } from "./components/CatalogPanel";
import { SkillWorkspace, type SourceFile } from "./components/SkillWorkspace";
import { RepositoryConnectionBar } from "./components/RepositoryConnectionBar";

const DEFAULT_SOURCE_URL = "https://github.com/baby-hero-404/prompt_base.git";

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
  const [showSetupGuide, setShowSetupGuide] = useState(false);
  
  const sourceUrlError = useMemo(() => validateSkillRepoURL(newSourceURL), [newSourceURL]);

  const { data: rawSkills, mutate: mutateSkills, isLoading: isLoadingSkills } = useAuthedSWR<Skill[]>(
    token ? ["global-skills"] : null,
    (t) => api.listSkills(t),
  );
  const skills = useMemo(() => rawSkills || [], [rawSkills]);

  const { data: rawSources, mutate: mutateSources, isLoading: isLoadingSources } = useAuthedSWR<SkillSource[]>(
    token ? ["skill-sources"] : null,
    (t) => api.listSkillSources(t),
  );
  const sources = useMemo(() => rawSources || [], [rawSources]);

  useEffect(() => {
    if (sources.length === 0 && !newSourceURL.trim()) {
      const timer = setTimeout(() => {
        setNewSourceURL(DEFAULT_SOURCE_URL);
      }, 0);
      return () => clearTimeout(timer);
    }
  }, [sources.length, newSourceURL]);

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

  const activeSkill = selectedSkill ?? filteredSkills[0] ?? null;
  const activeFolderPath = selectedSkill ? currentFolderPath : activeSkill ? folderForSkill(activeSkill) : "";
  const activeFilePath = selectedSkill ? selectedFilePath : activeSkill ? fileForSkill(activeSkill) : "";

  const selectedSourceID = activeSkill ? sources.find((item) => repoNameFromURL(item.url) === parseSkillMeta(activeSkill).repo)?.id ?? "" : "";

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
        const files = await api.listSkillSourceFiles(selectedSourceID, activeFolderPath, token);
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
  }, [activeFolderPath, selectedSourceID, token]);

  useEffect(() => {
    if (!token || !selectedSourceID || !activeFilePath) {
      const timer = setTimeout(() => {
        setFileContent("");
      }, 0);
      return () => clearTimeout(timer);
    }

    let isActive = true;

    async function loadContent() {
      setIsLoadingContent(true);
      try {
        const result = await api.getSkillSourceFileContent(selectedSourceID, activeFilePath, token);
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
  }, [activeFilePath, selectedSourceID, token]);

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
    if (sourceUrlError) {
      toast.error(sourceUrlError);
      return;
    }

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

      <div className="mx-auto w-full max-w-[1600px] space-y-6">
        <div className="flex flex-col gap-4 md:flex-row md:items-end md:justify-between">
          <div>
            <h2 className="font-mono text-2xl font-semibold text-foreground">Skills</h2>
            <p className="mt-1 max-w-2xl text-sm text-content-muted">
              Connect a skill repository, sync agent capabilities, and inspect source files before using them in workflows.
            </p>
            <button
              type="button"
              onClick={() => setShowSetupGuide(!showSetupGuide)}
              className="mt-3 inline-flex items-center gap-1.5 rounded border border-stroke bg-surface px-2.5 py-1 text-[11px] font-semibold text-foreground transition hover:bg-surface/85 cursor-pointer"
            >
              {showSetupGuide ? "Hide Setup Guide" : "Show Setup Guide"}
            </button>
          </div>
          <div className="grid grid-cols-3 gap-2 text-right text-xs">
            <div className="min-w-20 rounded-md border border-stroke bg-panel px-3 py-2">
              <div className="font-mono text-sm font-semibold text-foreground">{skills.length}</div>
              <div className="mt-0.5 text-[10px] uppercase tracking-wide text-content-muted">Skills</div>
            </div>
            <div className="min-w-20 rounded-md border border-stroke bg-panel px-3 py-2">
              <div className="font-mono text-sm font-semibold text-foreground">{categoryCount}</div>
              <div className="mt-0.5 text-[10px] uppercase tracking-wide text-content-muted">Categories</div>
            </div>
            <div className="min-w-20 rounded-md border border-stroke bg-panel px-3 py-2">
              <div className="font-mono text-sm font-semibold text-foreground">{`${syncedSourceCount}/${sourceCount}`}</div>
              <div className="mt-0.5 text-[10px] uppercase tracking-wide text-content-muted">Sources</div>
            </div>
          </div>
        </div>

        {/* Top Banner Control for Repository Connection */}
        {isLoadingSources ? (
          <div className="rounded-lg border border-stroke bg-card/60 p-4">
            <LoadingState label="Loading repository status..." />
          </div>
        ) : (
          <RepositoryConnectionBar
            sources={sources}
            newSourceURL={newSourceURL}
            setNewSourceURL={setNewSourceURL}
            isAddingSource={isAddingSource}
            syncingSourceID={syncingSourceID}
            deletingSourceID={deletingSourceID}
            sourceUrlError={sourceUrlError}
            onAddSource={addSource}
            onSyncSource={syncSource}
            onDeleteSource={deleteSource}
            onUseDefault={() => setNewSourceURL(DEFAULT_SOURCE_URL)}
          />
        )}

        {showSetupGuide && <SkillsGuide />}

        <div className="grid min-h-[720px] items-start gap-5 xl:grid-cols-[360px_330px_minmax(0,1fr)]">
          <CatalogPanel
            skills={filteredSkills}
            selectedSkillID={activeSkill?.id ?? ""}
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
            selectedSkill={activeSkill}
            currentFolderPath={activeFolderPath}
            files={filesList}
            selectedFilePath={activeFilePath}
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
      </div>
    </DashboardLayout>
  );
}
