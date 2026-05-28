"use client";

import useSWR from "swr";
import { Zap } from "lucide-react";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api } from "@/lib/api";
import type { Skill } from "@/lib/types";

export default function SkillsPage() {
  const session = useSession();
  const { data: skills = [] } = useSWR(
    session ? ["skills", session.token] : null,
    ([, token]) => api.listSkills(token),
  );

  return (
    <DashboardLayout>
      <div className="mb-5">
        <h2 className="font-mono text-2xl font-semibold">Skills</h2>
        <p className="mt-1 text-sm text-[var(--muted)]">
          Reusable capabilities agents can perform during task execution.
        </p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {skills.map((skill: Skill) => (
          <article
            key={skill.id}
            className="group rounded-lg border border-[var(--border)] bg-[var(--primary)] p-5 transition hover:border-[var(--accent)]/40"
          >
            <div className="mb-3 flex items-center gap-3">
              <div className="grid size-9 place-items-center rounded-md bg-[var(--accent)]/10 text-[var(--accent)]">
                <Zap size={18} />
              </div>
              <h3 className="font-mono font-semibold">{skill.name}</h3>
            </div>
            <p className="text-sm text-[var(--muted)]">{skill.description || "No description"}</p>
          </article>
        ))}
        {skills.length === 0 && (
          <p className="col-span-full text-sm text-[var(--muted)]">
            No skills configured yet. Skills are auto-seeded when you create a project.
          </p>
        )}
      </div>
    </DashboardLayout>
  );
}
