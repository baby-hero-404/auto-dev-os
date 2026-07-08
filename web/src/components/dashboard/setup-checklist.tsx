"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import {
  Key,
  GitBranch,
  FolderGit,
  Bot,
  ShieldCheck,
  Sparkles,
  Zap,
  X,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import { api } from "@/lib/api";
import { CheckItem } from "./checklist/types";
import { ChecklistItem } from "./checklist/ChecklistItem";
import { ChecklistSkeleton } from "./checklist/ChecklistSkeleton";

const LS_DISMISSED_KEY = "setup-checklist-dismissed";
const LS_AUTO_COMPLETED_KEY = "setup-checklist-auto-completed";

export function SetupChecklist({
  onCreateProjectClick,
}: {
  onCreateProjectClick?: () => void;
} = {}) {
  const session = useSession();
  const orgID = session?.user.org_id ?? "";

  const [dismissed, setDismissed] = useState(false);
  const [autoCompleted, setAutoCompleted] = useState(false);
  const [isExpanded, setIsExpanded] = useState(true);
  const [animatingId, setAnimatingId] = useState<string | null>(null);
  const prevDoneRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    const timer = setTimeout(() => {
      let wasDismissed = false;
      let wasAutoCompleted = false;

      try {
        wasDismissed = localStorage.getItem(LS_DISMISSED_KEY) === "true";
        wasAutoCompleted = localStorage.getItem(LS_AUTO_COMPLETED_KEY) === "true";
      } catch {
        // Keep first-run defaults when localStorage is unavailable.
      }

      setDismissed(wasDismissed);
      setAutoCompleted(wasAutoCompleted);
    }, 0);

    return () => clearTimeout(timer);
  }, []);

  // ─── Data fetching ────────────────────────────────────────
  const { data: credentials = [], isLoading: isCredLoading } = useAuthedSWR(
    orgID ? ["provider-credentials", orgID] : null,
    (t) => api.listProviderCredentials(orgID, t),
  );

  const { data: gitAccounts = [], isLoading: isGitLoading } = useAuthedSWR(
    orgID ? ["git-accounts", orgID] : null,
    (t) => api.listGitAccounts(orgID, t),
  );

  const { data: projects = [], isLoading: isProjectsLoading } = useAuthedSWR(
    orgID ? ["projects", orgID] : null,
    (t) => api.listProjects(orgID, t),
  );

  const { data: orgAgents = [], isLoading: isAgentsLoading } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  const { data: overview, isLoading: isOverviewLoading } = useAuthedSWR(
    orgID ? ["analytics-overview", orgID] : null,
    (t) => api.analyticsOverview(t, orgID),
  );

  const { data: globalRules = [], isLoading: isGlobalRulesLoading } = useAuthedSWR(
    orgID ? ["global-rules", orgID] : null,
    (t) => api.listGlobalRules(orgID, t),
  );

  const { data: skills = [], isLoading: isSkillsLoading } = useAuthedSWR(
    session?.token ? ["global-skills"] : null,
    (t) => api.listSkills(t),
  );

  const isLoading =
    isCredLoading ||
    isGitLoading ||
    isProjectsLoading ||
    isAgentsLoading ||
    isOverviewLoading ||
    isGlobalRulesLoading ||
    isSkillsLoading;

  // ─── Check computation ────────────────────────────────────
  const checks: CheckItem[] = useMemo(() => {
    const hasProvider = (credentials?.length ?? 0) > 0;
    const hasGit = (gitAccounts?.length ?? 0) > 0;
    const hasProject = (projects?.length ?? 0) > 0;
    const hasAgent = (orgAgents?.length ?? 0) > 0;
    const hasGlobalRules = (globalRules?.length ?? 0) > 0;
    const hasSkills = (skills?.length ?? 0) > 0;
    const hasTask = (overview?.total_tasks ?? 0) > 0;

    return [
      {
        id: "provider",
        label: "Add an AI provider key",
        href: "/ai-providers",
        hrefLabel: "AI Providers",
        icon: Key,
        required: true,
        done: hasProvider,
      },
      {
        id: "rules",
        label: "Configure organization rules",
        href: "/rules",
        hrefLabel: "Rules",
        icon: ShieldCheck,
        required: true,
        done: hasGlobalRules,
      },
      {
        id: "skills",
        label: "Seed or add global skills",
        href: "/skills",
        hrefLabel: "Skills",
        icon: Sparkles,
        required: true,
        done: hasSkills,
      },
      {
        id: "agent",
        label: "Add an agent to your organization",
        href: "/agents",
        hrefLabel: "Agents",
        icon: Bot,
        required: true,
        done: hasAgent,
      },
      {
        id: "git",
        label: "Connect a Git account",
        href: "/git-accounts",
        hrefLabel: "Git Accounts",
        icon: GitBranch,
        required: true,
        done: hasGit,
      },
      {
        id: "project",
        label: "Create a project",
        href: "/",
        hrefLabel: "Projects",
        icon: FolderGit,
        required: true,
        done: hasProject,
        onClick: onCreateProjectClick,
      },
      {
        id: "task",
        label: "Create your first task",
        href: hasProject ? `/projects/${projects[0].id}` : "/",
        hrefLabel: "Open a project",
        icon: Zap,
        required: true,
        done: hasTask,
      },
    ];
  }, [credentials, gitAccounts, projects, orgAgents, globalRules, skills, overview, onCreateProjectClick]);

  const requiredChecks = checks.filter((c) => c.required);
  const completedCount = checks.filter((c) => c.done).length;
  const requiredAllDone = requiredChecks.every((c) => c.done);

  // ─── Auto-hide when all required pass ─────────────────────
  useEffect(() => {
    if (requiredAllDone && !autoCompleted && !isLoading) {
      try {
        localStorage.setItem(LS_AUTO_COMPLETED_KEY, "true");
      } catch {
        // Keep the in-memory state even when localStorage is unavailable.
      }
      const timer = setTimeout(() => setAutoCompleted(true), 0);
      return () => clearTimeout(timer);
    }
  }, [requiredAllDone, autoCompleted, isLoading]);

  // ─── Animate newly completed checks ──────────────────────
  useEffect(() => {
    const currentDone = new Set(checks.filter((c) => c.done).map((c) => c.id));
    for (const id of currentDone) {
      if (!prevDoneRef.current.has(id)) {
        setAnimatingId(id);
        const timer = setTimeout(() => setAnimatingId(null), 400);
        prevDoneRef.current = currentDone;
        return () => clearTimeout(timer);
      }
    }
    prevDoneRef.current = currentDone;
  }, [checks]);

  // ─── Dismiss handler ──────────────────────────────────────
  function handleDismiss() {
    try {
      localStorage.setItem(LS_DISMISSED_KEY, "true");
    } catch {
      // Dismiss for this session if localStorage cannot persist the preference.
    }
    setDismissed(true);
  }

  // ─── Render conditions ────────────────────────────────────
  if (dismissed || autoCompleted) return null;

  return (
    <div className="mb-6 animate-fade-in">
      <div className="relative overflow-hidden rounded-xl border border-stroke bg-card shadow-lg">
        {/* Gradient accent strip */}
        <div className="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-brand-primary via-info to-brand-primary" />

        {/* Header */}
        <div className="flex items-center justify-between px-5 pt-5 pb-3">
          <div className="flex items-center gap-3">
            <div className="grid size-9 place-items-center rounded-lg bg-brand-primary-muted">
              <Zap size={18} className="text-brand-primary" />
            </div>
            <div>
              <h3 className="font-heading text-sm font-semibold text-foreground">
                Getting Started
              </h3>
              <p className="text-xs text-content-muted">
                {isLoading
                  ? "Loading setup status..."
                  : `${completedCount} of ${checks.length} complete`}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setIsExpanded((prev) => !prev)}
              className="rounded-md p-1.5 text-content-muted transition hover:bg-surface hover:text-foreground"
              title={isExpanded ? "Collapse" : "Expand"}
              type="button"
            >
              {isExpanded ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
            </button>
            <button
              onClick={handleDismiss}
              className="rounded-md p-1.5 text-content-muted transition hover:bg-surface hover:text-foreground"
              title="Dismiss setup checklist"
              type="button"
            >
              <X size={16} />
            </button>
          </div>
        </div>

        {/* Progress bar */}
        <div className="mx-5 mb-3">
          <div className="h-1.5 overflow-hidden rounded-full bg-surface">
            <div
              className="h-full rounded-full bg-brand-primary transition-all duration-500 ease-out"
              style={{ width: isLoading ? "0%" : `${(completedCount / checks.length) * 100}%` }}
            />
          </div>
        </div>

        {/* Checklist items */}
        {isExpanded && (
          <div className="px-5 pb-5">
            {isLoading ? (
              <ChecklistSkeleton />
            ) : (
              <div className="grid gap-1">
                {checks.map((check) => (
                  <ChecklistItem
                    key={check.id}
                    check={check}
                    isAnimating={animatingId === check.id}
                  />
                ))}
              </div>
            )}

            {/* Footer message */}
            {!isLoading && !requiredAllDone && (
              <p className="mt-4 text-center text-xs text-content-muted">
                Complete the required steps to run your first AI task.
              </p>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
