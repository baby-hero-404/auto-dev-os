"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import {
  CheckCircle2,
  Circle,
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

const LS_DISMISSED_KEY = "setup-checklist-dismissed";
const LS_AUTO_COMPLETED_KEY = "setup-checklist-auto-completed";

type CheckItem = {
  id: string;
  label: string;
  href: string;
  hrefLabel: string;
  icon: typeof Key;
  required: boolean;
  done: boolean;
};

export function SetupChecklist() {
  const session = useSession();
  const orgID = session?.user.org_id ?? "";

  const [dismissed, setDismissed] = useState(true); // start hidden, reveal after mount
  const [autoCompleted, setAutoCompleted] = useState(false);
  const [isExpanded, setIsExpanded] = useState(true);
  const [animatingId, setAnimatingId] = useState<string | null>(null);
  const prevDoneRef = useRef<Set<string>>(new Set());

  // Hydrate dismiss state from localStorage after mount
  useEffect(() => {
    const wasDismissed = localStorage.getItem(LS_DISMISSED_KEY) === "true";
    const wasAutoCompleted = localStorage.getItem(LS_AUTO_COMPLETED_KEY) === "true";
    const timer = setTimeout(() => {
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
  }, [credentials, gitAccounts, projects, orgAgents, globalRules, skills, overview]);

  const requiredChecks = checks.filter((c) => c.required);
  const completedCount = checks.filter((c) => c.done).length;
  const requiredAllDone = requiredChecks.every((c) => c.done);

  // ─── Auto-hide when all required pass ─────────────────────
  useEffect(() => {
    if (requiredAllDone && !autoCompleted && !isLoading) {
      localStorage.setItem(LS_AUTO_COMPLETED_KEY, "true");
      const timer = setTimeout(() => {
        setAutoCompleted(true);
      }, 0);
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
    localStorage.setItem(LS_DISMISSED_KEY, "true");
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

function ChecklistItem({
  check,
  isAnimating,
}: {
  check: CheckItem;
  isAnimating: boolean;
}) {
  const Icon = check.icon;

  return (
    <Link
      href={check.href}
      className={`group flex items-center gap-3 rounded-lg px-3 py-2.5 transition hover:bg-surface ${
        check.done ? "opacity-70" : ""
      }`}
    >
      {/* Status indicator */}
      <span className={isAnimating ? "animate-completion-pop" : ""}>
        {check.done ? (
          <CheckCircle2
            size={18}
            className={check.required ? "text-success" : "text-warning"}
          />
        ) : (
          <Circle size={18} className="text-content-muted" />
        )}
      </span>

      {/* Icon */}
      <span className="grid size-7 shrink-0 place-items-center rounded-md bg-surface">
        <Icon size={14} className="text-content-muted group-hover:text-brand-primary transition" />
      </span>

      {/* Label */}
      <span
        className={`flex-1 text-sm transition ${
          check.done
            ? "text-content-muted line-through decoration-content-muted/40"
            : "text-foreground group-hover:text-brand-primary"
        }`}
      >
        {check.label}
        {!check.required && (
          <span className="ml-1.5 text-[10px] font-semibold uppercase tracking-wider text-warning">
            recommended
          </span>
        )}
      </span>

      {/* Link hint */}
      {!check.done && (
        <span className="hidden text-xs text-content-muted group-hover:text-brand-primary sm:inline transition">
          {check.hrefLabel} →
        </span>
      )}
    </Link>
  );
}

function ChecklistSkeleton() {
  return (
    <div className="grid gap-1">
      {[0, 1, 2, 3, 4, 5, 6, 7].map((i) => (
        <div key={i} className="flex items-center gap-3 rounded-lg px-3 py-2.5">
          <div className="skeleton-shimmer size-[18px] rounded-full" />
          <div className="skeleton-shimmer size-7 rounded-md" />
          <div className="skeleton-shimmer h-4 flex-1 rounded" />
        </div>
      ))}
    </div>
  );
}
