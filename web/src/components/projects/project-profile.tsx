import { FormEvent, useEffect, useState } from "react";
import { Save, Settings } from "lucide-react";
import { ApiError } from "@/lib/api";

interface ProjectProfileProps {
  initialName: string;
  initialDescription: string;
  onUpdateProject: (name: string, description: string) => Promise<void>;
}

export function ProjectProfile({
  initialName,
  initialDescription,
  onUpdateProject,
}: ProjectProfileProps) {
  const [name, setName] = useState(initialName);
  const [description, setDescription] = useState(initialDescription);
  const [isUpdating, setIsUpdating] = useState(false);
  const [updateError, setUpdateError] = useState("");

  useEffect(() => {
    setName(initialName);
    setDescription(initialDescription);
  }, [initialName, initialDescription]);

  async function handleUpdateProject(e: FormEvent) {
    e.preventDefault();
    setUpdateError("");
    setIsUpdating(true);
    try {
      await onUpdateProject(name.trim(), description.trim());
    } catch (err) {
      setUpdateError(err instanceof ApiError ? err.message : "Failed to update project");
    } finally {
      setIsUpdating(false);
    }
  }

  return (
    <div className="rounded-lg border border-stroke bg-card p-5 max-w-2xl">
      <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-3">
        <Settings size={18} className="text-brand-primary" />
        <h3 className="font-sans font-semibold text-foreground">General Settings</h3>
      </div>
      <form onSubmit={handleUpdateProject} className="space-y-4">
        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Project Name</label>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none transition-all"
            required
            disabled={isUpdating}
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Description</label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="min-h-[100px] rounded border border-stroke bg-surface px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none resize-none transition-all"
            disabled={isUpdating}
          />
        </div>
        {updateError && <p className="text-xs text-red-400">{updateError}</p>}
        <button
          type="submit"
          disabled={isUpdating}
          className="flex items-center gap-2 rounded bg-brand-primary px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
        >
          <Save size={16} />
          {isUpdating ? "Saving..." : "Save Project Settings"}
        </button>
      </form>
    </div>
  );
}
