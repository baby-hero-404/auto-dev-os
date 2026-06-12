"use client";

import { useState, FormEvent } from "react";
import { Zap, Plus, Edit3, Trash2, Loader2, X, Code } from "lucide-react";
import { toast, Toaster } from "sonner";
import { DashboardLayout } from "@/components/dashboard/dashboard-layout";
import { useSession } from "@/lib/session";
import { api, ApiError } from "@/lib/api";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Skill } from "@/lib/types";

export default function SkillsPage() {
  const session = useSession();
  const [isSeeding, setIsSeeding] = useState(false);

  // Modal states
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [modalMode, setModalMode] = useState<"create" | "edit">("create");
  const [editingSkillID, setEditingSkillID] = useState("");
  const [skillName, setSkillName] = useState("");
  const [skillDescription, setSkillDescription] = useState("");
  const [skillSchema, setSkillSchema] = useState("{\n  \"source\": \"custom\"\n}");
  const [validationError, setValidationError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Delete states
  const [confirmingDeleteSkillID, setConfirmingDeleteSkillID] = useState("");
  const [isDeleting, setIsDeleting] = useState(false);

  const token = session?.token ?? "";

  const { data: skills = [], mutate } = useAuthedSWR(
    token ? ["global-skills"] : null,
    (t) => api.listSkills(t),
  );

  async function handleSeed() {
    if (!token) return;
    setIsSeeding(true);
    try {
      await api.seedSkills(token);
      toast.success("Default skills populated!");
      mutate();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to seed skills");
    } finally {
      setIsSeeding(false);
    }
  }

  function openCreateModal() {
    setModalMode("create");
    setEditingSkillID("");
    setSkillName("");
    setSkillDescription("");
    setSkillSchema("{\n  \"source\": \"custom\"\n}");
    setValidationError("");
    setIsModalOpen(true);
  }

  function openEditModal(skill: Skill) {
    setModalMode("edit");
    setEditingSkillID(skill.id);
    setSkillName(skill.name);
    setSkillDescription(skill.description || "");
    try {
      const parsed = typeof skill.schema === "string" ? JSON.parse(skill.schema) : skill.schema;
      setSkillSchema(JSON.stringify(parsed, null, 2));
    } catch {
      setSkillSchema(typeof skill.schema === "string" ? skill.schema : "{}");
    }
    setValidationError("");
    setIsModalOpen(true);
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;

    const name = skillName.trim();
    if (!name) {
      setValidationError("Name is required");
      return;
    }

    let parsedSchema: Record<string, unknown> = {};
    try {
      const parsed = JSON.parse(skillSchema.trim() || "{}");
      if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
        setValidationError("Schema must be a valid JSON object");
        return;
      }
      parsedSchema = parsed as Record<string, unknown>;
    } catch {
      setValidationError("Schema must be a valid JSON object");
      return;
    }

    setValidationError("");
    setIsSubmitting(true);
    try {
      if (modalMode === "create") {
        await api.createSkill(token, {
          name,
          description: skillDescription.trim(),
          schema: parsedSchema,
        });
        toast.success(`Skill "${name}" created successfully`);
      } else {
        await api.updateSkill(editingSkillID, token, {
          name,
          description: skillDescription.trim(),
          schema: parsedSchema,
        });
        toast.success(`Skill "${name}" updated successfully`);
      }
      setIsModalOpen(false);
      mutate();
    } catch (err) {
      setValidationError(err instanceof ApiError ? err.message : "An error occurred");
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleDelete(skillID: string) {
    if (!token) return;
    setIsDeleting(true);
    try {
      await api.deleteSkill(skillID, token);
      toast.success("Skill deleted successfully");
      setConfirmingDeleteSkillID("");
      mutate();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete skill");
    } finally {
      setIsDeleting(false);
    }
  }

  return (
    <DashboardLayout>
      <Toaster closeButton position="top-right" richColors />
      <div className="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-2xl font-semibold text-foreground">Skills</h2>
          <p className="mt-1 text-sm text-content-muted">
            Reusable capabilities agents can perform during task execution.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="hidden rounded-lg border border-stroke bg-card p-3 text-brand-primary shadow-sm sm:grid sm:size-12 sm:place-items-center">
            <Zap size={22} />
          </div>
          <div className="flex gap-2">
            <button
              onClick={openCreateModal}
              className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
              type="button"
            >
              <Plus size={16} />
              Create Skill
            </button>
            {skills.length === 0 && (
              <button
                onClick={handleSeed}
                disabled={isSeeding}
                className="inline-flex items-center gap-2 rounded-md border border-stroke bg-card px-3 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                type="button"
              >
                {isSeeding ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
                Fill Default Skills
              </button>
            )}
          </div>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {skills.map((skill: Skill) => {
          let hasMetadata = false;
          let category = "";
          let registry = "";
          try {
            const parsed = typeof skill.schema === "string" ? JSON.parse(skill.schema) : skill.schema;
            if (parsed.category || parsed.registry) {
              hasMetadata = true;
              category = parsed.category || "";
              registry = parsed.registry || "";
            }
          } catch {}

          return (
            <article
              key={skill.id}
              className="group relative rounded-lg border border-stroke bg-card p-5 shadow-sm transition hover:border-brand-primary/40"
            >
              {/* Card actions (visible on hover) */}
              <div className="absolute top-4 right-4 flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
                <button
                  onClick={() => openEditModal(skill)}
                  className="rounded-md p-1.5 text-content-muted transition hover:bg-surface hover:text-foreground cursor-pointer"
                  title="Edit skill"
                  type="button"
                >
                  <Edit3 size={14} />
                </button>
                <button
                  onClick={() => setConfirmingDeleteSkillID(skill.id)}
                  className="rounded-md p-1.5 text-content-muted transition hover:bg-danger/10 hover:text-danger cursor-pointer"
                  title="Delete skill"
                  type="button"
                >
                  <Trash2 size={14} />
                </button>
              </div>

              <div className="mb-3 flex items-center gap-3">
                <div className="grid size-9 place-items-center rounded-md bg-brand-primary/10 text-brand-primary">
                  <Zap size={18} />
                </div>
                <h3 className="font-mono font-semibold text-foreground pr-14 truncate" title={skill.name}>
                  {skill.name}
                </h3>
              </div>
              <p className="text-sm text-content-muted line-clamp-3 mb-4">{skill.description || "No description"}</p>

              {hasMetadata && (
                <div className="mt-auto flex flex-wrap items-center gap-2 pt-2 border-t border-stroke/40">
                  {category && (
                    <span className="rounded border border-blue-500/20 bg-blue-500/10 px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-blue-300">
                      {category}
                    </span>
                  )}
                  {registry && (
                    <span className="rounded border border-purple-500/20 bg-purple-500/10 px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-purple-300">
                      {registry}
                    </span>
                  )}
                </div>
              )}

              {/* Confirm Delete Overlay inside Card */}
              {confirmingDeleteSkillID === skill.id && (
                <div className="absolute inset-0 z-10 flex flex-col items-center justify-center rounded-lg bg-card/95 p-4 text-center backdrop-blur-xs border border-danger/20">
                  <p className="text-xs font-semibold text-foreground mb-3">Delete skill &quot;{skill.name}&quot;?</p>
                  <div className="flex gap-2">
                    <button
                      onClick={() => handleDelete(skill.id)}
                      disabled={isDeleting}
                      className="inline-flex items-center gap-1 rounded bg-danger px-3 py-1 text-xs font-semibold text-white transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                      type="button"
                    >
                      {isDeleting && <Loader2 size={12} className="animate-spin" />}
                      Delete
                    </button>
                    <button
                      onClick={() => setConfirmingDeleteSkillID("")}
                      disabled={isDeleting}
                      className="rounded border border-stroke px-3 py-1 text-xs font-semibold text-foreground transition hover:bg-surface disabled:opacity-50 cursor-pointer"
                      type="button"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              )}
            </article>
          );
        })}

        {skills.length === 0 && (
          <div className="col-span-full rounded-lg border border-dashed border-stroke bg-panel/50 p-10 text-center">
            <p className="font-mono text-sm font-semibold text-foreground">No skills configured yet.</p>
            <p className="mt-2 text-sm text-content-muted max-w-md mx-auto">
              Skills define standard tools and workflows that AI agents can execute. Start by seeding the default capabilities or create your own custom skill.
            </p>
            <div className="mt-6 flex justify-center gap-3">
              <button
                onClick={openCreateModal}
                className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 cursor-pointer"
                type="button"
              >
                <Plus size={16} />
                Create Custom Skill
              </button>
              <button
                onClick={handleSeed}
                disabled={isSeeding}
                className="inline-flex items-center gap-2 rounded-md border border-stroke bg-card px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface cursor-pointer disabled:opacity-50"
                type="button"
              >
                {isSeeding ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
                Seed default skills
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Slide-over / Modal for Create/Edit Skill */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/70 backdrop-blur-xs">
          <div className="relative w-full max-w-lg rounded-lg border border-stroke bg-card p-6 shadow-2xl animate-in fade-in zoom-in duration-200">
            <button
              onClick={() => setIsModalOpen(false)}
              className="absolute top-4 right-4 rounded-md p-1.5 text-content-muted hover:bg-surface hover:text-foreground transition cursor-pointer"
              type="button"
            >
              <X size={16} />
            </button>

            <h3 className="font-mono text-lg font-bold text-foreground mb-4 flex items-center gap-2">
              <Code size={18} className="text-brand-primary" />
              {modalMode === "create" ? "Create Custom Skill" : `Edit Skill: ${skillName}`}
            </h3>

            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <label className="block text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-1.5">
                  Skill Name *
                </label>
                <input
                  type="text"
                  value={skillName}
                  onChange={(e) => setSkillName(e.target.value)}
                  placeholder="e.g. read_system_logs"
                  className="w-full rounded border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20"
                  disabled={isSubmitting}
                  required
                />
              </div>

              <div>
                <label className="block text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-1.5">
                  Description
                </label>
                <textarea
                  value={skillDescription}
                  onChange={(e) => setSkillDescription(e.target.value)}
                  placeholder="Describe what this skill enables the agent to do..."
                  className="w-full min-h-[60px] rounded border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:outline-none focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20 resize-none"
                  disabled={isSubmitting}
                />
              </div>

              <div>
                <label className="block text-xs font-mono font-bold uppercase tracking-wider text-content-muted mb-1.5">
                  Schema (JSON Object) *
                </label>
                <textarea
                  value={skillSchema}
                  onChange={(e) => setSkillSchema(e.target.value)}
                  placeholder='{ "source": "custom" }'
                  className="w-full min-h-[140px] font-mono text-xs rounded border border-stroke bg-background px-3 py-2 text-foreground focus:outline-none focus:border-brand-primary focus:ring-2 focus:ring-brand-primary/20"
                  disabled={isSubmitting}
                  required
                />
              </div>

              {validationError && (
                <p className="rounded border border-danger/20 bg-danger/10 p-2.5 text-xs text-danger font-medium">
                  {validationError}
                </p>
              )}

              <div className="flex justify-end gap-2 pt-2">
                <button
                  onClick={() => setIsModalOpen(false)}
                  disabled={isSubmitting}
                  className="rounded border border-stroke px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface disabled:opacity-50 cursor-pointer"
                  type="button"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isSubmitting || !skillName.trim()}
                  className="inline-flex items-center justify-center gap-2 rounded bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                >
                  {isSubmitting ? <Loader2 size={16} className="animate-spin" /> : null}
                  {modalMode === "create" ? "Create Skill" : "Save Changes"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </DashboardLayout>
  );
}
