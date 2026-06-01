"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  Brain,
  ThumbsUp,
  ThumbsDown,
  Info,
  Loader2,
  FileCode,
  ChevronLeft,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { EmptyState } from "@/components/ui/empty-state";
import type { LearningSuggestion } from "@/lib/types";
import Link from "next/link";

export default function SuggestionsPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [selectedAgentID, setSelectedAgentID] = useState<string>("");
  const [selectedStatus, setSelectedStatus] = useState<string>("pending");
  const [rejectionID, setRejectionID] = useState<string | null>(null);
  const [rejectionFeedback, setRejectionFeedback] = useState<string>("");
  const [actioningID, setActioningID] = useState<string | null>(null);

  // Fetch all agents in organization staff pool
  const { data: orgAgents = [], isLoading: loadingAgents } = useSWR(
    session ? ["org-agents", orgID, token] : null,
    ([, oid, t]) => api.listOrgAgents(oid, t),
  );

  const activeAgentID = selectedAgentID || orgAgents[0]?.id || "";

  // Fetch suggestions of selected agent
  const {
    data: suggestionData,
    mutate: mutateSuggestions,
    isLoading: loadingSuggestions,
  } = useSWR(
    session && activeAgentID
      ? ["suggestions", activeAgentID, selectedStatus, token]
      : null,
    ([, aid, status, t]) => api.listSuggestions(aid, t, status),
  );

  const suggestionsList = suggestionData?.suggestions ?? [];
  const selectedAgent = orgAgents.find((a) => a.id === activeAgentID);

  async function handleApprove(id: string) {
    if (!confirm("Are you sure you want to approve and apply this suggestion?")) {
      return;
    }
    setActioningID(id);
    try {
      await api.approveSuggestion(id, token);
      mutateSuggestions();
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Failed to approve suggestion");
    } finally {
      setActioningID(null);
    }
  }

  async function handleRejectSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!rejectionID) return;

    setActioningID(rejectionID);
    try {
      await api.rejectSuggestion(rejectionID, token, rejectionFeedback);
      setRejectionID(null);
      setRejectionFeedback("");
      mutateSuggestions();
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Failed to reject suggestion");
    } finally {
      setActioningID(null);
    }
  }

  return (
    <DashboardLayout>
      <div className="mb-6 flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <Link
            href="/knowledge"
            className="flex items-center gap-1 text-xs font-mono text-[var(--muted)] hover:text-white transition"
          >
            <ChevronLeft size={14} />
            Back to Memory Browser
          </Link>
        </div>
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <h2 className="font-mono text-2xl font-semibold flex items-center gap-2 text-white">
              <Brain className="text-[var(--accent)]" />
              HITL Learning & Self-Improvement Loop
            </h2>
            <p className="mt-1 text-sm text-[var(--muted)]">
              Approve or reject automated optimization rules, prompts patches, and skill playbooks.
            </p>
          </div>
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
                    onClick={() => setSelectedAgentID(agent.id)}
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
        </aside>

        {/* Right Pane: Suggestions Queue */}
        <main className="flex flex-col gap-4">
          {/* Status Tabs */}
          <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex gap-1.5 overflow-x-auto pb-1 sm:pb-0">
              {["pending", "approved", "rejected", "applied"].map((status) => (
                <button
                  key={status}
                  onClick={() => {
                    setSelectedStatus(status);
                    setRejectionID(null);
                  }}
                  className={`rounded px-3 py-1.5 text-xs font-mono border transition cursor-pointer capitalize ${
                    selectedStatus === status
                      ? "bg-[var(--accent)]/15 border-[var(--accent)] text-[var(--accent)]"
                      : "bg-slate-950 border-[var(--border)] text-[var(--muted)] hover:text-slate-200"
                  }`}
                >
                  {status}
                </button>
              ))}
            </div>

            {selectedAgent && (
              <span className="text-xs font-mono text-[var(--muted)]">
                Agent: <span className="text-slate-200">{selectedAgent.name}</span>
              </span>
            )}
          </div>

          {/* Suggestions List */}
          {loadingSuggestions ? (
            <div className="flex flex-col items-center justify-center py-20 text-[var(--muted)] bg-[var(--primary)] border border-[var(--border)] rounded-lg">
              <Loader2 size={32} className="animate-spin mb-3 text-[var(--accent)]" />
              <p className="text-sm font-mono">Fetching suggestion records...</p>
            </div>
          ) : suggestionsList.length === 0 ? (
            <EmptyState
              icon={Brain}
              title={`No ${selectedStatus} suggestions`}
              description={`Suggestions will appear here when agents fail tasks (for Prompts Patches) or succeed (for Rules/Patterns).`}
            />
          ) : (
            <div className="space-y-4">
              {suggestionsList.map((suggestion) => {
                const confidencePct = Math.round(suggestion.confidence * 100);
                const isPending = suggestion.status === "pending";
                const feedback = suggestionFeedback(suggestion);

                return (
                  <article
                    key={suggestion.id}
                    className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 flex flex-col gap-4 transition hover:border-[var(--accent)]/30"
                  >
                    <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
                      <div>
                        <div className="flex flex-wrap items-center gap-2 mb-2">
                          <span className="rounded bg-indigo-500/10 border border-indigo-500/20 px-2 py-0.5 text-[10px] font-mono text-indigo-300 font-bold uppercase tracking-wider">
                            {suggestion.suggestion_type}
                          </span>
                          <span className="text-xs text-[var(--muted)] font-mono">
                            ID: {suggestion.id.slice(0, 8)}...
                          </span>
                        </div>
                        <h3 className="font-mono text-sm font-bold text-white">{suggestion.title}</h3>
                        <p className="mt-1.5 text-xs text-[var(--muted)] leading-relaxed">
                          {suggestion.description}
                        </p>
                      </div>

                      {/* Confidence Score Gauge */}
                      <div className="flex flex-col items-center sm:items-end justify-center min-w-[100px]">
                        <span className="text-[10px] font-mono text-[var(--muted)] uppercase">Confidence</span>
                        <div className="mt-1 flex items-center gap-2">
                          <div className="h-2 w-16 bg-slate-900 border border-[var(--border)] rounded-full overflow-hidden">
                            <div
                              className="h-full bg-emerald-400 rounded-full"
                              style={{ width: `${confidencePct}%` }}
                            ></div>
                          </div>
                          <span className="font-mono text-xs font-bold text-emerald-400">{confidencePct}%</span>
                        </div>
                      </div>
                    </div>

                    {/* Content Section (Rule/Prompt Patch body) */}
                    <div>
                      <div className="mb-1 flex items-center gap-1.5 text-[10px] font-mono font-bold uppercase tracking-wider text-[var(--muted)]">
                        <FileCode size={12} />
                        Proposed Content
                      </div>
                      <pre className="rounded bg-slate-950 border border-[var(--border)] p-3.5 font-mono text-xs text-slate-300 overflow-x-auto whitespace-pre-wrap">
                        {suggestion.content}
                      </pre>
                    </div>

                    {/* Applied Metadata or Feedback Info */}
                    {suggestion.status === "rejected" && feedback && (
                      <div className="rounded border border-red-500/20 bg-red-950/20 p-3 text-xs text-red-200 flex items-start gap-2">
                        <Info size={16} className="text-red-400 mt-0.5 shrink-0" />
                        <div>
                          <span className="font-bold">Rejection Feedback:</span> {feedback}
                        </div>
                      </div>
                    )}

                    {suggestion.status === "applied" && feedback && (
                      <div className="rounded border border-emerald-500/20 bg-emerald-950/20 p-3 text-xs text-emerald-200 flex items-start gap-2">
                        <Info size={16} className="text-emerald-400 mt-0.5 shrink-0" />
                        <div>
                          <span className="font-bold">Execution details:</span> {feedback}
                        </div>
                      </div>
                    )}

                    {/* Pending Action Buttons */}
                    {isPending && rejectionID !== suggestion.id && (
                      <div className="flex gap-2 justify-end border-t border-[var(--border)]/60 pt-4 mt-1">
                        <button
                          onClick={() => setRejectionID(suggestion.id)}
                          className="rounded-md border border-red-500/20 bg-red-950/10 px-4 py-2 text-xs font-semibold text-red-300 hover:bg-red-950/30 transition cursor-pointer flex items-center gap-1"
                          disabled={actioningID === suggestion.id}
                        >
                          <ThumbsDown size={14} />
                          Reject Suggestion
                        </button>
                        <button
                          onClick={() => handleApprove(suggestion.id)}
                          className="rounded-md bg-emerald-400 px-4 py-2 text-xs font-semibold text-slate-950 hover:bg-emerald-300 transition cursor-pointer flex items-center gap-1"
                          disabled={actioningID === suggestion.id}
                        >
                          {actioningID === suggestion.id ? (
                            <Loader2 size={14} className="animate-spin" />
                          ) : (
                            <ThumbsUp size={14} />
                          )}
                          Approve & Apply
                        </button>
                      </div>
                    )}

                    {/* Rejection Form Overlay */}
                    {rejectionID === suggestion.id && (
                      <form
                        onSubmit={handleRejectSubmit}
                        className="border-t border-[var(--border)]/60 pt-4 mt-1 flex flex-col gap-3"
                      >
                        <div className="flex flex-col gap-1.5">
                          <label className="text-xs text-[var(--muted)] font-mono">
                            Provide Feedback (Reason for rejection)
                          </label>
                          <textarea
                            value={rejectionFeedback}
                            onChange={(e) => setRejectionFeedback(e.target.value)}
                            placeholder="Why is this suggestion invalid? (e.g. incorrect pattern, conflicts with rule X)"
                            className="rounded border border-[var(--border)] bg-slate-950 px-3 py-2 text-xs text-white h-20 focus:outline-none focus:border-red-500"
                            required
                          />
                        </div>
                        <div className="flex justify-end gap-2">
                          <button
                            type="button"
                            onClick={() => {
                              setRejectionID(null);
                              setRejectionFeedback("");
                            }}
                            className="rounded border border-[var(--border)] bg-slate-900 px-3 py-1.5 text-xs text-slate-300 hover:text-white cursor-pointer"
                          >
                            Cancel
                          </button>
                          <button
                            type="submit"
                            className="rounded bg-red-500 px-3 py-1.5 text-xs font-semibold text-white hover:bg-red-400 transition cursor-pointer"
                          >
                            {actioningID === suggestion.id ? (
                              <Loader2 size={12} className="animate-spin" />
                            ) : (
                              "Submit Rejection"
                            )}
                          </button>
                        </div>
                      </form>
                    )}
                  </article>
                );
              })}
            </div>
          )}
        </main>
      </div>
    </DashboardLayout>
  );
}

function suggestionFeedback(suggestion: LearningSuggestion) {
  const feedback = suggestion.metadata?.review_feedback;
  if (typeof feedback === "string") return feedback;
  return "";
}
