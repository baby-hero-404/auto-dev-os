"use client";

import { useState } from "react";
import useSWR, { mutate } from "swr";
import Link from "next/link";
import { GraduationCap, CheckCircle, Ban, Trash2, ExternalLink, Loader2, Sparkles } from "lucide-react";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { learnedSkills as learnedSkillsApi } from "@/lib/api";
import type { LearnedSkill } from "@/lib/types";
import { toast } from "sonner";

interface LearnedSkillsPanelProps {
  projectID: string;
  token: string;
}

export function LearnedSkillsPanel({ projectID, token }: LearnedSkillsPanelProps) {
  const [filterStatus, setFilterStatus] = useState<"all" | "draft" | "active" | "disabled">("all");
  const [actionID, setActionID] = useState<string | null>(null);

  const swrKey = projectID && token ? [`/projects/${projectID}/learned-skills`, token] : null;
  const { data: skills, isLoading } = useSWR<LearnedSkill[]>(swrKey, () =>
    learnedSkillsApi.listByProject(projectID, token)
  );

  const filteredSkills = (skills || []).filter((s) => {
    if (filterStatus === "all") return true;
    return s.status === filterStatus;
  });

  async function handleApprove(skillID: string) {
    setActionID(skillID);
    try {
      await learnedSkillsApi.update(skillID, token, { status: "active" });
      toast.success("Learned skill approved and activated.");
      mutate(swrKey);
    } catch (err) {
      toast.error("Failed to approve skill: " + (err instanceof Error ? err.message : "Unknown error"));
    } finally {
      setActionID(null);
    }
  }

  async function handleDisable(skillID: string) {
    setActionID(skillID);
    try {
      await learnedSkillsApi.update(skillID, token, { status: "disabled" });
      toast.info("Learned skill disabled.");
      mutate(swrKey);
    } catch (err) {
      toast.error("Failed to disable skill: " + (err instanceof Error ? err.message : "Unknown error"));
    } finally {
      setActionID(null);
    }
  }

  async function handleDelete(skillID: string) {
    if (!confirm("Are you sure you want to delete this learned skill? This action cannot be undone.")) {
      return;
    }
    setActionID(skillID);
    try {
      await learnedSkillsApi.remove(skillID, token);
      toast.success("Learned skill deleted.");
      mutate(swrKey);
    } catch (err) {
      toast.error("Failed to delete skill: " + (err instanceof Error ? err.message : "Unknown error"));
    } finally {
      setActionID(null);
    }
  }

  return (
    <Card>
      <CardHeader
        title="Learned Skills (extracted from merged tasks)"
        icon={<GraduationCap size={18} className="text-brand-primary" />}
      />
      <CardContent className="space-y-4">
        {/* Filter buttons */}
        <div className="flex items-center gap-1.5 border-b border-stroke/10 pb-3">
          {(["all", "draft", "active", "disabled"] as const).map((st) => (
            <button
              key={st}
              type="button"
              onClick={() => setFilterStatus(st)}
              className={`px-3 py-1 text-xs font-semibold rounded-lg capitalize transition cursor-pointer ${
                filterStatus === st
                  ? "bg-brand-primary text-brand-primary-fg shadow-sm"
                  : "bg-slate-500/10 text-content-muted hover:text-foreground"
              }`}
            >
              {st}
            </button>
          ))}
        </div>

        {isLoading ? (
          <div className="p-4 text-xs font-mono text-content-muted flex items-center gap-2">
            <Loader2 size={14} className="animate-spin text-brand-primary" /> Loading learned skills...
          </div>
        ) : filteredSkills.length === 0 ? (
          <div className="p-6 text-center text-xs text-content-muted italic bg-slate-500/[0.02] rounded-xl border border-stroke/10">
            <Sparkles size={24} className="mx-auto mb-2 opacity-40 text-brand-primary" />
            No learned skills found in status &quot;{filterStatus}&quot;.
          </div>
        ) : (
          <div className="divide-y divide-stroke/10 border border-stroke/10 rounded-xl overflow-hidden bg-slate-500/[0.02]">
            {filteredSkills.map((skill) => (
              <div key={skill.id} className="p-4 space-y-2 hover:bg-slate-500/5 transition">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <h4 className="font-heading text-xs font-bold text-foreground">{skill.title}</h4>
                    <span
                      className={`text-[9px] font-bold uppercase px-2 py-0.5 rounded-full ${
                        skill.status === "active"
                          ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border border-emerald-500/20"
                          : skill.status === "draft"
                          ? "bg-amber-500/10 text-amber-600 dark:text-amber-400 border border-amber-500/20"
                          : "bg-slate-500/10 text-content-muted"
                      }`}
                    >
                      {skill.status}
                    </span>
                  </div>

                  <div className="flex items-center gap-1.5">
                    {skill.status === "draft" && (
                      <Button
                        type="button"
                        size="sm"
                        onClick={() => handleApprove(skill.id)}
                        disabled={actionID === skill.id}
                        isLoading={actionID === skill.id}
                        className="h-7 px-2.5 text-[11px] bg-emerald-600 hover:bg-emerald-700 text-white"
                      >
                        <CheckCircle size={12} /> Approve
                      </Button>
                    )}
                    {skill.status === "active" && (
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() => handleDisable(skill.id)}
                        disabled={actionID === skill.id}
                        isLoading={actionID === skill.id}
                        className="h-7 px-2.5 text-[11px]"
                      >
                        <Ban size={12} /> Disable
                      </Button>
                    )}
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={() => handleDelete(skill.id)}
                      disabled={actionID === skill.id}
                      className="h-7 px-2 text-[11px] text-danger hover:bg-danger/10"
                    >
                      <Trash2 size={12} />
                    </Button>
                  </div>
                </div>

                {/* Trigger keywords */}
                {skill.trigger_keywords && skill.trigger_keywords.length > 0 && (
                  <div className="flex flex-wrap gap-1 items-center text-[10px]">
                    <span className="text-content-muted font-medium">Triggers:</span>
                    {skill.trigger_keywords.map((kw, i) => (
                      <span
                        key={i}
                        className="bg-brand-primary/10 text-brand-primary font-mono px-1.5 py-0.5 rounded border border-brand-primary/20"
                      >
                        {kw}
                      </span>
                    ))}
                  </div>
                )}

                {/* Content snippet */}
                {skill.content && (
                  <p className="text-xs font-mono bg-slate-950 text-slate-300 p-2.5 rounded-lg whitespace-pre-wrap max-h-24 overflow-y-auto custom-scrollbar border border-stroke/10">
                    {skill.content}
                  </p>
                )}

                {/* Metrics & Source task link */}
                <div className="flex flex-wrap items-center justify-between text-[10px] text-content-muted font-mono pt-1">
                  <span>
                    Used: <strong className="text-foreground">{skill.usage_count}</strong> | Success:{" "}
                    <strong className="text-foreground">{skill.success_count}</strong>
                  </span>
                  {skill.source_task_id && (
                    <Link
                      href={`/projects/${projectID}/tasks/${skill.source_task_id}`}
                      className="inline-flex items-center gap-1 text-brand-primary hover:underline"
                    >
                      Source Task <ExternalLink size={10} />
                    </Link>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
