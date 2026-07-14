"use client";

import { useState } from "react";
import useSWR from "swr";
import { Brain, Database, Loader2, Search } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { EmptyState } from "@/components/ui/empty-state";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import type { MemorySearchResult } from "@/lib/types";
import Link from "next/link";

import { MemoryCard } from "./components/MemoryCard";
import { AgentConfigSidebar } from "./components/AgentConfigSidebar";
import { MemoryInspector } from "./components/MemoryInspector";
import { SearchBar } from "./components/SearchBar";

export default function KnowledgePage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";
  const isAdmin = session?.user.role === "admin";

  const [selectedAgentID, setSelectedAgentID] = useState<string>("");
  const [selectedTier, setSelectedTier] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [isSearching, setIsSearching] = useState<boolean>(false);
  const [searchResults, setSearchResults] = useState<MemorySearchResult[] | null>(null);
  const [inspectingMemoryID, setInspectingMemoryID] = useState<string | null>(null);
  const [deleteMemoryId, setDeleteMemoryId] = useState<string | null>(null);

  // Fetch all agents in organization staff pool
  const { data: orgAgents = [], isLoading: loadingAgents } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  const activeAgentID = selectedAgentID || orgAgents[0]?.id || "";

  // Fetch memories of selected agent (regular list)
  const {
    data: memoryData,
    mutate: mutateMemories,
    isLoading: loadingMemories,
  } = useSWR(
    session && activeAgentID && !searchQuery
      ? ["memories", activeAgentID, selectedTier]
      : null,
    () => api.listMemories(activeAgentID, token, selectedTier),
  );

  // Inspect specific memory detail and edges
  const { data: detailData } = useSWR(
    session && inspectingMemoryID ? ["memory-detail", inspectingMemoryID] : null,
    () => api.getMemory(inspectingMemoryID!, token),
  );

  const memoriesList = memoryData?.memories ?? [];
  const selectedAgent = orgAgents.find((a) => a.id === activeAgentID);

  async function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    if (!activeAgentID || !searchQuery.trim()) {
      setSearchResults(null);
      return;
    }
    setIsSearching(true);
    try {
      const resp = await api.searchMemories(activeAgentID, searchQuery, token);
      setSearchResults(resp.results || []);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Search failed");
    } finally {
      setIsSearching(false);
    }
  }

  function handleClearSearch() {
    setSearchQuery("");
    setSearchResults(null);
  }

  async function performDeleteMemory(memoryID: string) {
    try {
      await api.deleteMemory(memoryID, token);
      mutateMemories();
      if (inspectingMemoryID === memoryID) {
        setInspectingMemoryID(null);
      }
      if (searchResults) {
        setSearchResults((prev) => prev?.filter((item) => item.memory.id !== memoryID) ?? null);
      }
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Failed to delete memory");
    }
  }

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
        <div>
          <h2 className="font-mono text-2xl font-semibold">Knowledge & Memory</h2>
          <p className="mt-1 text-sm text-content-muted">
            Explore 4-Tier episodic and semantic agent memories promoting from working to procedural knowledge.
          </p>
        </div>
        <div className="flex gap-2">
          <Link
            href="/knowledge/suggestions"
            className="flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
          >
            <Brain size={16} />
            Learning Loop Queue
          </Link>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[250px_1fr]">
        <AgentConfigSidebar
          loadingAgents={loadingAgents}
          orgAgents={orgAgents}
          activeAgentID={activeAgentID}
          selectedAgent={selectedAgent}
          onSelectAgent={(id) => {
            setSelectedAgentID(id);
            handleClearSearch();
          }}
        />

        <main className="flex flex-col gap-4">
          <SearchBar
            searchQuery={searchQuery}
            isSearching={isSearching}
            selectedTier={selectedTier}
            onSearchChange={setSearchQuery}
            onSearchSubmit={handleSearch}
            onClearSearch={handleClearSearch}
            onSelectTier={setSelectedTier}
          />

          {searchQuery ? (
            <div>
              <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-3 flex items-center gap-1.5">
                <Database size={14} />
                Hybrid Triple-Stream RRF Search Results
              </h3>
              {searchResults === null ? (
                <div className="rounded-lg border border-stroke bg-panel p-8 text-center text-content-muted">
                  Press enter or click search to run hybrid rank query.
                </div>
              ) : searchResults.length === 0 ? (
                <EmptyState
                  icon={Search}
                  title="No matching memories"
                  description="Try adjusting your query term or filter settings."
                />
              ) : (
                <div className="grid gap-4 md:grid-cols-2">
                  {searchResults.map((item) => (
                    <MemoryCard
                      key={item.memory.id}
                      memory={item.memory}
                      score={item.final_score}
                      isAdmin={isAdmin}
                      onDelete={setDeleteMemoryId}
                      onInspect={setInspectingMemoryID}
                      isInspecting={inspectingMemoryID === item.memory.id}
                    />
                  ))}
                </div>
              )}
            </div>
          ) : (
            <div>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted flex items-center gap-1.5">
                  <Database size={14} />
                  Memory Entries
                </h3>
                {selectedTier && (
                  <span className="text-xs font-mono text-brand-primary">
                    Filtered: <span className="capitalize">{selectedTier}</span>
                  </span>
                )}
              </div>
              {loadingMemories ? (
                <div className="flex flex-col items-center justify-center py-20 text-content-muted bg-panel border border-stroke rounded-lg">
                  <Loader2 size={32} className="animate-spin mb-3 text-brand-primary" />
                  <p className="text-sm font-mono">Fetching memory database...</p>
                </div>
              ) : memoriesList.length === 0 ? (
                <EmptyState
                  icon={Brain}
                  title={`No ${selectedTier || ""} memories yet`}
                  description="Run agent task workflows to generate observations, errors, and episodic Prompts patches."
                />
              ) : (
                <div className="grid gap-4 md:grid-cols-2">
                  {memoriesList.map((mem) => (
                    <MemoryCard
                      key={mem.id}
                      memory={mem}
                      isAdmin={isAdmin}
                      onDelete={setDeleteMemoryId}
                      onInspect={setInspectingMemoryID}
                      isInspecting={inspectingMemoryID === mem.id}
                    />
                  ))}
                </div>
              )}
            </div>
          )}

          {inspectingMemoryID && (
            <MemoryInspector
              detailData={detailData}
              onClose={() => setInspectingMemoryID(null)}
            />
          )}
        </main>
      </div>

      <ConfirmDialog
        isOpen={deleteMemoryId !== null}
        title="Prune Memory"
        description="Are you sure you want to delete this episodic memory item? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        onConfirm={() => {
          if (deleteMemoryId) {
            performDeleteMemory(deleteMemoryId);
          }
        }}
        onClose={() => setDeleteMemoryId(null)}
      />
    </DashboardLayout>
  );
}
