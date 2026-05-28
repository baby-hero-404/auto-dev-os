"use client";

import { BookOpen, FileText, GitBranch } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";

export default function KnowledgePage() {
  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Knowledge</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">
          Agent context sources and reference documents.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        <KnowledgeCard
          icon={FileText}
          title="Architecture"
          description="System architecture documentation including tech stack, domain models, and dependency maps."
          status="Active"
        />
        <KnowledgeCard
          icon={GitBranch}
          title="Git Context"
          description="Repository metadata, branch history, and recent commits used for task analysis."
          status="Active"
        />
        <KnowledgeCard
          icon={BookOpen}
          title="Learning Resources"
          description="Reference projects and learning reports from Multica, 9Router, Hermes Agent, and more."
          status="Reference"
        />
      </div>

      <div className="mt-6 rounded-lg border border-dashed border-[var(--border)] bg-[var(--primary)]/50 p-8 text-center">
        <BookOpen size={32} className="mx-auto mb-3 text-[var(--muted)]" />
        <h3 className="font-mono font-semibold">Episodic Memory</h3>
        <p className="mt-2 text-sm text-[var(--muted)]">
          Vector-based episodic memory with pgvector is planned for Phase 6.
          <br />
          Agents will learn from past task executions and improve over time.
        </p>
      </div>
    </DashboardLayout>
  );
}

function KnowledgeCard({
  icon: Icon,
  title,
  description,
  status,
}: {
  icon: React.ComponentType<{ size?: number; className?: string }>;
  title: string;
  description: string;
  status: string;
}) {
  return (
    <div className="rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]/40">
      <div className="mb-3 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Icon size={18} className="text-[var(--accent)]" />
          <h3 className="font-mono font-semibold">{title}</h3>
        </div>
        <span className="rounded bg-slate-800 px-2 py-1 text-xs text-slate-200">{status}</span>
      </div>
      <p className="text-sm text-[var(--muted)]">{description}</p>
    </div>
  );
}
