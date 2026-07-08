"use client";

import { useState } from "react";
import useSWR from "swr";
import {
  Brain,
  Loader2,
  ChevronLeft,
} from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { EmptyState } from "@/components/ui/empty-state";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import Link from "next/link";
import { SuggestionCard } from "./components/SuggestionCard";

export default function SuggestionsPage() {
  const session = useSession();
  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const [selectedAgentID, setSelectedAgentID] = useState<string>("");
  const [selectedStatus, setSelectedStatus] = useState<string>("pending");
  const [rejectionID, setRejectionID] = useState<string | null>(null);
  const [rejectionFeedback, setRejectionFeedback] = useState<string>("");
  const [actioningID, setActioningID] = useState<string | null>(null);
  const [approveSuggestionId, setApproveSuggestionId] = useState<string | null>(null);

  // Fetch all agents in organization staff pool
  const { data: orgAgents = [], isLoading: loadingAgents } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  const activeAgentID = selectedAgentID || orgAgents[0]?.id || "";

  // Fetch suggestions of selected agent
  const {
    data: suggestionData,
    mutate: mutateSuggestions,
    isLoading: loadingSuggestions,
  } = useSWR(
    session && activeAgentID
      ? ["suggestions", activeAgentID, selectedStatus]
      : null,
    () => api.listSuggestions(activeAgentID, token, selectedStatus),
  );

  const suggestionsList = suggestionData?.suggestions ?? [];
  const selectedAgent = orgAgents.find((a) => a.id === activeAgentID);

  async function performApprove(id: string) {
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
            className="flex items-center gap-1 text-xs font-mono text-content-muted hover:text-white transition"
          >
            <ChevronLeft size={14} />
            Back to Memory Browser
          </Link>
        </div>
        <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <h2 className="font-mono text-2xl font-semibold flex items-center gap-2 text-white">
              <Brain className="text-brand-primary" />
              HITL Learning & Self-Improvement Loop
            </h2>
            <p className="mt-1 text-sm text-content-muted">
              Approve or reject automated optimization rules, prompts patches, and skill playbooks.
            </p>
          </div>
        </div>
      </div>

      <div className="grid gap-6 lg:grid-cols-[250px_1fr]">
        {/* Left Sidebar: Agent Selector */}
        <aside className="rounded-lg border border-stroke bg-panel p-4 flex flex-col gap-4">
          <div>
            <h3 className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-2">
              Select Agent
            </h3>
            {loadingAgents ? (
              <div className="flex items-center gap-2 text-sm text-content-muted py-2">
                <Loader2 size={16} className="animate-spin" />
                Loading agents...
              </div>
            ) : orgAgents.length === 0 ? (
              <p className="text-xs text-content-muted italic">No agents available.</p>
            ) : (
              <div className="space-y-1">
                {orgAgents.map((agent) => (
                  <button
                    key={agent.id}
                    onClick={() => setSelectedAgentID(agent.id)}
                    className={`w-full text-left rounded-md px-3 py-2 text-xs font-mono flex items-center justify-between transition cursor-pointer ${
                      activeAgentID === agent.id
                        ? "bg-brand-primary text-slate-950 font-bold"
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
          <div className="rounded-lg border border-stroke bg-panel p-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
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
                      ? "bg-brand-primary/15 border-brand-primary text-brand-primary"
                      : "bg-slate-950 border-stroke text-content-muted hover:text-slate-200"
                  }`}
                >
                  {status}
                </button>
              ))}
            </div>

            {selectedAgent && (
              <span className="text-xs font-mono text-content-muted">
                Agent: <span className="text-slate-200">{selectedAgent.name}</span>
              </span>
            )}
          </div>

          {/* Suggestions List */}
          {loadingSuggestions ? (
            <div className="flex flex-col items-center justify-center py-20 text-content-muted bg-panel border border-stroke rounded-lg">
              <Loader2 size={32} className="animate-spin mb-3 text-brand-primary" />
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
              {suggestionsList.map((suggestion) => (
                <SuggestionCard
                  key={suggestion.id}
                  suggestion={suggestion}
                  actioningID={actioningID}
                  rejectionID={rejectionID}
                  setRejectionID={setRejectionID}
                  rejectionFeedback={rejectionFeedback}
                  setRejectionFeedback={setRejectionFeedback}
                  onRejectSubmit={handleRejectSubmit}
                  onApproveClick={setApproveSuggestionId}
                />
              ))}
            </div>
          )}
        </main>
      </div>
      <ConfirmDialog
        isOpen={approveSuggestionId !== null}
        title="Approve & Apply Suggestion"
        description="Are you sure you want to approve and apply this suggestion? This will immediately apply the optimization rule or prompt patch."
        confirmText="Approve"
        variant="info"
        onConfirm={() => {
          if (approveSuggestionId) {
            performApprove(approveSuggestionId);
          }
        }}
        onClose={() => setApproveSuggestionId(null)}
      />
    </DashboardLayout>
  );
}
