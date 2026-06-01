"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  Brain,
  Search,
  Database,
  Trash2,
  Calendar,
  Network,
  TrendingDown,
  Loader2,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { EmptyState } from "@/components/ui/empty-state";
import type { EpisodicMemory, KnowledgeEdge, MemorySearchResult } from "@/lib/types";
import Link from "next/link";

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

  // Fetch all agents in organization staff pool
  const { data: orgAgents = [], isLoading: loadingAgents } = useSWR(
    session ? ["org-agents", orgID, token] : null,
    ([, oid, t]) => api.listOrgAgents(oid, t),
  );

  const activeAgentID = selectedAgentID || orgAgents[0]?.id || "";

  // Fetch memories of selected agent (regular list)
  const {
    data: memoryData,
    mutate: mutateMemories,
    isLoading: loadingMemories,
  } = useSWR(
    session && activeAgentID && !searchQuery
      ? ["memories", activeAgentID, selectedTier, token]
      : null,
    ([, aid, tier, t]) => api.listMemories(aid, t, tier),
  );

  // Inspect specific memory detail and edges
  const { data: detailData } = useSWR(
    session && inspectingMemoryID ? ["memory-detail", inspectingMemoryID, token] : null,
    ([, mid, t]) => api.getMemory(mid, t),
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

  async function handleDeleteMemory(memoryID: string) {
    if (!confirm("Are you sure you want to delete this episodic memory item?")) {
      return;
    }
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
          <p className="mt-1 text-sm text-[var(--muted)]">
            Explore 4-Tier episodic and semantic agent memories promoting from working to procedural knowledge.
          </p>
        </div>
        <div className="flex gap-2">
          <Link
            href="/knowledge/suggestions"
            className="flex items-center gap-2 rounded-md bg-[var(--accent)] px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
          >
            <Brain size={16} />
            Learning Loop Queue
          </Link>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[250px_1fr]">
        {/* Left Sidebar: Agent Selector */}
        <aside className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-4 flex flex-col gap-4">
          <div>
            <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-[var(--muted)] mb-2">
              Select Agent
            </h3>
            {loadingAgents ? (
              <div className="flex items-center gap-2 text-sm text-[var(--muted)] py-2">
                <Loader2 size={16} className="animate-spin" />
                Loading agents...
              </div>
            ) : orgAgents.length === 0 ? (
              <p className="text-xs text-[var(--muted)] italic">No agents available.</p>
            ) : (
              <div className="space-y-1">
                {orgAgents.map((agent) => (
                  <button
                    key={agent.id}
                    onClick={() => {
                      setSelectedAgentID(agent.id);
                      handleClearSearch();
                    }}
                    className={`w-full text-left rounded-md px-3 py-2 text-xs font-mono flex items-center justify-between transition cursor-pointer ${
                      activeAgentID === agent.id
                        ? "bg-[var(--accent)] text-slate-950 font-bold"
                        : "text-slate-300 hover:bg-slate-800"
                    }`}
                  >
                    <span>{agent.name}</span>
                    <span className="opacity-70 text-[9px] uppercase">{agent.role}</span>
                  </button>
                ))}
              </div>
            )}
          </div>

          {selectedAgent && (
            <div className="border-t border-[var(--border)] pt-4">
              <h4 className="text-xs font-mono font-bold uppercase tracking-wider text-[var(--muted)] mb-2">
                Agent Config
              </h4>
              <div className="space-y-1.5 text-xs">
                <div className="flex justify-between">
                  <span className="text-[var(--muted)]">Model:</span>
                  <span className="font-mono text-slate-200">{selectedAgent.model}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-[var(--muted)]">Level:</span>
                  <span className="font-mono text-slate-200 capitalize">{selectedAgent.level}</span>
                </div>
              </div>
            </div>
          )}
        </aside>

        {/* Right Pane: Memories & Search */}
        <main className="flex flex-col gap-4">
          {/* Filters & Search */}
          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <form onSubmit={handleSearch} className="flex-1 flex gap-2 max-w-lg">
              <div className="relative flex-1">
                <Search size={16} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--muted)]" />
                <input
                  type="text"
                  placeholder="Triple-Stream Search (BM25 + Vector + Graph)..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="w-full rounded-md border border-[var(--border)] bg-slate-950 pl-9 pr-3 py-2 text-sm text-white focus:outline-none focus:border-[var(--accent)]"
                />
              </div>
              <button
                type="submit"
                disabled={isSearching}
                className="rounded-md bg-slate-900 border border-[var(--border)] px-4 py-2 text-sm font-semibold text-white hover:bg-slate-800 transition cursor-pointer flex items-center gap-1.5"
              >
                {isSearching ? <Loader2 size={14} className="animate-spin" /> : "Search"}
              </button>
              {searchQuery && (
                <button
                  type="button"
                  onClick={handleClearSearch}
                  className="rounded-md border border-red-500/20 bg-red-950/20 px-3 py-2 text-sm font-semibold text-red-300 hover:bg-red-950/40 transition cursor-pointer"
                >
                  Clear
                </button>
              )}
            </form>

            {!searchQuery && (
              <div className="flex gap-1.5 overflow-x-auto pb-1 sm:pb-0">
                {["", "working", "episodic", "semantic", "procedural"].map((tier) => (
                  <button
                    key={tier}
                    onClick={() => setSelectedTier(tier)}
                    className={`rounded px-2.5 py-1 text-xs font-mono border transition cursor-pointer capitalize ${
                      selectedTier === tier
                        ? "bg-[var(--accent)]/15 border-[var(--accent)] text-[var(--accent)]"
                        : "bg-slate-950 border-[var(--border)] text-[var(--muted)] hover:text-slate-200"
                    }`}
                  >
                    {tier || "All Tiers"}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* Results Grid / List */}
          {searchQuery ? (
            // Search Results View
            <div>
              <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-[var(--muted)] mb-3 flex items-center gap-1.5">
                <Database size={14} />
                Hybrid Triple-Stream RRF Search Results
              </h3>
              {searchResults === null ? (
                <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-8 text-center text-[var(--muted)]">
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
                      onDelete={handleDeleteMemory}
                      onInspect={setInspectingMemoryID}
                      isInspecting={inspectingMemoryID === item.memory.id}
                    />
                  ))}
                </div>
              )}
            </div>
          ) : (
            // Normal browsing list
            <div>
              <div className="mb-3 flex items-center justify-between">
                <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-[var(--muted)] flex items-center gap-1.5">
                  <Database size={14} />
                  Memory Entries
                </h3>
                {selectedTier && (
                  <span className="text-xs font-mono text-[var(--accent)]">
                    Filtered: <span className="capitalize">{selectedTier}</span>
                  </span>
                )}
              </div>
              {loadingMemories ? (
                <div className="flex flex-col items-center justify-center py-20 text-[var(--muted)] bg-[var(--primary)] border border-[var(--border)] rounded-lg">
                  <Loader2 size={32} className="animate-spin mb-3 text-[var(--accent)]" />
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
                      onDelete={handleDeleteMemory}
                      onInspect={setInspectingMemoryID}
                      isInspecting={inspectingMemoryID === mem.id}
                    />
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Inspecting Memory Detail Modal/Drawer */}
          {inspectingMemoryID && (
            <div className="rounded-lg border border-[var(--border)] bg-slate-950 p-5 mt-4">
              <div className="mb-4 flex items-center justify-between border-b border-[var(--border)] pb-3">
                <h3 className="font-mono font-semibold flex items-center gap-2 text-white">
                  <Network size={16} className="text-[var(--accent)]" />
                  Memory Inspector & Relations Graph
                </h3>
                <button
                  onClick={() => setInspectingMemoryID(null)}
                  className="text-xs text-[var(--muted)] hover:text-white cursor-pointer"
                >
                  ✕ Close
                </button>
              </div>

              {!detailData ? (
                <div className="flex justify-center py-8">
                  <Loader2 size={24} className="animate-spin text-[var(--accent)]" />
                </div>
              ) : (
                <div className="grid gap-6 md:grid-cols-[1fr_250px]">
                  <div className="flex flex-col gap-3">
                    <div>
                      <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)]">Summary</span>
                      <p className="text-sm text-slate-100 font-medium">{detailData.memory.summary}</p>
                    </div>
                    <div>
                      <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)]">Content Details</span>
                      <pre className="mt-1 rounded bg-slate-900 border border-[var(--border)] p-3 font-mono text-xs text-slate-300 overflow-x-auto whitespace-pre-wrap">
                        {detailData.memory.content}
                      </pre>
                    </div>
                    {detailData.memory.tags && detailData.memory.tags.length > 0 && (
                      <div>
                        <span className="text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)] block mb-1">Tags</span>
                        <div className="flex flex-wrap gap-1">
                          {detailData.memory.tags.map((t: string) => (
                            <span key={t} className="rounded bg-slate-800 border border-[var(--border)] px-1.5 py-0.5 text-[10px] text-slate-300 font-mono">
                              #{t}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="border-t border-[var(--border)] pt-4 md:border-t-0 md:pt-0 md:border-l md:pl-4 flex flex-col gap-4">
                    <div>
                      <h4 className="text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)] mb-2">
                        Connected Entities
                      </h4>
                      {!detailData.edges || detailData.edges.length === 0 ? (
                        <p className="text-xs text-[var(--muted)] italic">No knowledge connections detected.</p>
                      ) : (
                        <div className="space-y-2">
                          {detailData.edges.map((edge: KnowledgeEdge) => (
                            <div key={edge.id} className="rounded border border-[var(--border)] bg-slate-900/60 p-2 text-xs">
                              <div className="font-mono font-semibold text-[var(--accent)]">{edge.relation}</div>
                              <div className="text-[10px] text-[var(--muted)] mt-0.5">
                                Weight: {edge.weight.toFixed(2)}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="space-y-2 text-xs border-t border-[var(--border)] pt-3">
                      <div className="flex justify-between">
                        <span className="text-[var(--muted)]">Importance:</span>
                        <span className="font-mono text-slate-300 font-semibold">{detailData.memory.access_count}</span>
                      </div>
                      <div className="flex justify-between">
                        <span className="text-[var(--muted)]">Decay math:</span>
                        <span className="font-mono text-slate-300 font-semibold flex items-center gap-1">
                          <TrendingDown size={12} className="text-amber-400" />
                          {detailData.memory.decay_score.toFixed(3)}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </main>
      </div>
    </DashboardLayout>
  );
}

function MemoryCard({
  memory,
  score,
  isAdmin,
  onDelete,
  onInspect,
  isInspecting,
}: {
  memory: EpisodicMemory;
  score?: number;
  isAdmin: boolean;
  onDelete: (id: string) => void;
  onInspect: (id: string) => void;
  isInspecting: boolean;
}) {
  const isError = memory.category === "error";
  const isSuccess = memory.category === "success";

  const tierColors = {
    working: "border-slate-800 text-slate-300 bg-slate-900/40",
    episodic: "border-cyan-500/20 text-cyan-300 bg-cyan-950/10",
    semantic: "border-amber-500/20 text-amber-300 bg-amber-950/10",
    procedural: "border-emerald-500/20 text-emerald-300 bg-emerald-950/10",
  };

  return (
    <article
      className={`group rounded-lg border p-4 transition flex flex-col justify-between cursor-pointer ${
        isInspecting
          ? "border-[var(--accent)] bg-slate-950/80 shadow-[0_0_12px_rgba(235,166,90,0.15)]"
          : "border-[var(--border)] bg-[var(--primary)] hover:border-[var(--accent)]/40"
      }`}
      onClick={() => onInspect(memory.id)}
    >
      <div>
        <div className="mb-2 flex items-start justify-between">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className={`rounded px-1.5 py-0.5 text-[9px] font-mono font-bold uppercase tracking-wider border ${tierColors[memory.tier as keyof typeof tierColors] || "border-slate-800"}`}>
              {memory.tier}
            </span>
            <span className={`rounded-full px-2 py-0.5 text-[9px] font-mono ${
              isError
                ? "bg-red-400/10 text-red-300 border border-red-500/20"
                : isSuccess
                ? "bg-emerald-400/10 text-emerald-300 border border-emerald-500/20"
                : "bg-slate-800 text-slate-300 border border-slate-700/50"
            }`}>
              {memory.category}
            </span>
            {score !== undefined && (
              <span className="rounded bg-indigo-500/10 text-indigo-300 border border-indigo-500/20 px-1.5 py-0.5 text-[9px] font-mono">
                RRF Rnk: {score.toFixed(4)}
              </span>
            )}
          </div>
          {isAdmin && (
            <button
              onClick={(e) => {
                e.stopPropagation();
                onDelete(memory.id);
              }}
              className="rounded-md p-1 text-slate-500 opacity-0 transition hover:bg-red-950/40 hover:text-red-300 group-hover:opacity-100 cursor-pointer"
              title="Prune memory"
            >
              <Trash2 size={13} />
            </button>
          )}
        </div>

        <h4 className="font-mono text-xs font-semibold text-slate-100 line-clamp-2 mt-1">{memory.summary}</h4>
        <p className="mt-2 text-xs text-[var(--muted)] line-clamp-3 leading-relaxed">{memory.content}</p>
      </div>

      <div className="mt-3 border-t border-[var(--border)]/60 pt-2 flex items-center justify-between text-[10px] text-[var(--muted)]">
        <span className="flex items-center gap-1">
          <Calendar size={11} />
          {new Date(memory.created_at).toLocaleDateString()}
        </span>
        <span className="font-mono">Seen: {memory.access_count}</span>
      </div>
    </article>
  );
}
