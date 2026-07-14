import { FormEvent, useState } from "react";
import { Save, Settings, Bot, RefreshCw } from "lucide-react";
import { ApiError } from "@/lib/api";
import type { Project } from "@/lib/types";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select } from "@/components/ui/select";
import { Field } from "@/components/ui/field";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

interface ProjectProfileProps {
  project: Project | undefined;
  onUpdateProject: (input: {
    name?: string;
    description?: string;
    default_model_level?: string;
    default_autonomy?: string;
    auto_review_policy?: string;
    max_retries?: number;
    max_review_fix_cycles?: number;
    default_branch?: string;
  }) => Promise<void>;
}

export function ProjectProfile({ project, onUpdateProject }: ProjectProfileProps) {
  const [name, setName] = useState(project?.name ?? "");
  const [description, setDescription] = useState(project?.description ?? "");
  
  const [defaultModelLevel, setDefaultModelLevel] = useState(project?.default_model_level ?? "balanced");
  const [defaultAutonomy, setDefaultAutonomy] = useState(project?.default_autonomy ?? "supervised");
  const [autoReviewPolicy, setAutoReviewPolicy] = useState(project?.auto_review_policy ?? "complexity_based");
  const [maxRetries, setMaxRetries] = useState(project?.max_retries ?? 3);
  const [maxReviewFixCycles, setMaxReviewFixCycles] = useState(project?.max_review_fix_cycles ?? 3);
  const [defaultBranch, setDefaultBranch] = useState(project?.default_branch ?? "main");

  const [isUpdating, setIsUpdating] = useState(false);
  const [updateError, setUpdateError] = useState("");

  // Render-phase prop synchronization to avoid cascading renders
  const [prevProject, setPrevProject] = useState(project);
  if (project !== prevProject) {
    setPrevProject(project);
    if (project) {
      setName(project.name);
      setDescription(project.description);
      setDefaultModelLevel(project.default_model_level ?? "balanced");
      setDefaultAutonomy(project.default_autonomy ?? "supervised");
      setAutoReviewPolicy(project.auto_review_policy ?? "complexity_based");
      setMaxRetries(project.max_retries ?? 3);
      setMaxReviewFixCycles(project.max_review_fix_cycles ?? 3);
      setDefaultBranch(project.default_branch ?? "main");
    }
  }

  // Dirty state tracking (compares current inputs with stored project values)
  const isDirty =
    project?.name !== name ||
    project?.description !== description ||
    (project?.default_model_level ?? "balanced") !== defaultModelLevel ||
    (project?.default_autonomy ?? "supervised") !== defaultAutonomy ||
    (project?.auto_review_policy ?? "complexity_based") !== autoReviewPolicy ||
    (project?.max_retries ?? 3) !== maxRetries ||
    (project?.max_review_fix_cycles ?? 3) !== maxReviewFixCycles ||
    (project?.default_branch ?? "main") !== defaultBranch;

  function handleReset() {
    if (project) {
      setName(project.name);
      setDescription(project.description);
      setDefaultModelLevel(project.default_model_level ?? "balanced");
      setDefaultAutonomy(project.default_autonomy ?? "supervised");
      setAutoReviewPolicy(project.auto_review_policy ?? "complexity_based");
      setMaxRetries(project.max_retries ?? 3);
      setMaxReviewFixCycles(project.max_review_fix_cycles ?? 3);
      setDefaultBranch(project.default_branch ?? "main");
      setUpdateError("");
      toast.info("Project settings reverted.");
    }
  }

  async function handleUpdateProject(e: FormEvent) {
    e.preventDefault();
    setUpdateError("");
    setIsUpdating(true);
    try {
      await onUpdateProject({
        name: name.trim(),
        description: description.trim(),
        default_model_level: defaultModelLevel,
        default_autonomy: defaultAutonomy,
        auto_review_policy: autoReviewPolicy,
        max_retries: maxRetries,
        max_review_fix_cycles: maxReviewFixCycles,
        default_branch: defaultBranch.trim(),
      });
      toast.success("Project settings updated successfully.");
    } catch (err) {
      setUpdateError(err instanceof ApiError ? err.message : "Failed to update project");
      toast.error(err instanceof ApiError ? err.message : "Failed to update project");
    } finally {
      setIsUpdating(false);
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      <form onSubmit={handleUpdateProject} className="space-y-6">
        
        {/* General Settings */}
        <Card>
          <CardHeader
            title="General Settings"
            icon={<Settings size={18} className="text-brand-primary" />}
          />
          <CardContent className="space-y-4">
            <Field label="Project Name" htmlFor="profile-name">
              <Input
                id="profile-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
                disabled={isUpdating}
              />
            </Field>
            <Field label="Description" htmlFor="profile-desc">
              <Textarea
                id="profile-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                disabled={isUpdating}
                className="resize-none"
              />
            </Field>
            <Field label="Default Branch" htmlFor="profile-branch">
              <Input
                id="profile-branch"
                value={defaultBranch}
                onChange={(e) => setDefaultBranch(e.target.value)}
                placeholder="main"
                required
                disabled={isUpdating}
              />
            </Field>
          </CardContent>
        </Card>

        {/* AI Workflow Defaults */}
        <Card>
          <CardHeader
            title="AI Workflow Defaults"
            icon={<Bot size={18} className="text-brand-primary" />}
          />
          <CardContent className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <Field label="Model Level" htmlFor="profile-model">
              <Select
                id="profile-model"
                value={defaultModelLevel}
                onChange={(e) => setDefaultModelLevel(e.target.value)}
                disabled={isUpdating}
              >
                <option value="fast">Fast</option>
                <option value="balanced">Balanced</option>
                <option value="powerful">Powerful</option>
              </Select>
            </Field>
            <Field label="Autonomy" htmlFor="profile-autonomy">
              <Select
                id="profile-autonomy"
                value={defaultAutonomy}
                onChange={(e) => setDefaultAutonomy(e.target.value)}
                disabled={isUpdating}
              >
                <option value="supervised">Supervised (Requires Approval)</option>
                <option value="autonomous">Autonomous</option>
              </Select>
            </Field>
            <Field label="Review Policy" htmlFor="profile-policy">
              <Select
                id="profile-policy"
                value={autoReviewPolicy}
                onChange={(e) => setAutoReviewPolicy(e.target.value)}
                disabled={isUpdating}
              >
                <option value="complexity_based">Complexity Based</option>
                <option value="always_review">Always Review</option>
                <option value="auto_merge">Auto Merge (No Review)</option>
              </Select>
            </Field>
            <Field label="Max Retries" htmlFor="profile-retries">
              <Input
                id="profile-retries"
                type="number"
                min={0}
                max={10}
                value={maxRetries}
                onChange={(e) => setMaxRetries(Number(e.target.value))}
                required
                disabled={isUpdating}
              />
            </Field>
            <Field label="Max Review-Fix Cycles" htmlFor="profile-cycles">
              <Input
                id="profile-cycles"
                type="number"
                min={1}
                max={10}
                value={maxReviewFixCycles}
                onChange={(e) => setMaxReviewFixCycles(Number(e.target.value))}
                required
                disabled={isUpdating}
              />
            </Field>
          </CardContent>
        </Card>

        {updateError && (
          <span className="text-xs text-danger font-medium leading-normal block">{updateError}</span>
        )}

        {isDirty && (
          <div className="rounded-md bg-warning/10 border border-warning/20 p-3 text-xs text-warning">
            You have unsaved changes. Save or reset your changes before navigating away.
          </div>
        )}
        
        <div className="flex items-center gap-3">
          <Button
            type="submit"
            disabled={isUpdating || !isDirty}
            isLoading={isUpdating}
          >
            <Save size={16} />
            Save Project Settings
          </Button>

          {isDirty && (
            <Button
              type="button"
              variant="secondary"
              onClick={handleReset}
              disabled={isUpdating}
            >
              <RefreshCw size={14} />
              Reset Changes
            </Button>
          )}
        </div>
      </form>
    </div>
  );
}
