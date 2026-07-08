import { FormEvent, useState } from "react";
import { Loader2, Plus, X } from "lucide-react";

interface AddModelModalProps {
  level: "fast" | "balanced" | "powerful";
  onClose: () => void;
  onSubmit: (name: string, priority: number) => Promise<void>;
}

export function AddModelModal({
  level,
  onClose,
  onSubmit,
}: AddModelModalProps) {
  const [newModelName, setNewModelName] = useState("");
  const [newModelPriority, setNewModelPriority] = useState("0");
  const [isAddingModel, setIsAddingModel] = useState(false);

  async function handleFormSubmit(event: FormEvent) {
    event.preventDefault();
    if (!newModelName.trim()) return;
    setIsAddingModel(true);
    try {
      await onSubmit(newModelName.trim(), parseInt(newModelPriority, 10) || 0);
      setNewModelName("");
      setNewModelPriority("0");
      onClose();
    } catch {
      // Error handled by parent toast
    } finally {
      setIsAddingModel(false);
    }
  }

  return (
    <div
      className="fixed inset-0 z-modal grid place-items-center bg-black/45 px-4 py-6 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      onMouseDown={onClose}
    >
      <div
        className="glass-panel animate-modal-in w-full max-w-sm rounded-lg p-5 shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between gap-4">
          <div className="flex items-center gap-2">
            <Plus size={18} className="text-brand-primary" />
            <h3 className="font-semibold text-foreground">
              Add Model ({level})
            </h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground"
            title="Close"
          >
            <X size={16} />
          </button>
        </div>

        <form onSubmit={handleFormSubmit} className="space-y-4">
          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Model Name</span>
            <input
              required
              value={newModelName}
              onChange={(event) => setNewModelName(event.target.value)}
              placeholder="e.g. gpt-4o-mini"
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
            />
          </label>

          <label className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-3">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Priority</span>
              <span className="text-right text-[10px] font-medium text-content-muted">Lower = runs first (0 = highest)</span>
            </div>
            <input
              type="number"
              min="0"
              value={newModelPriority}
              onChange={(event) => setNewModelPriority(event.target.value)}
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
            />
          </label>

          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-foreground transition hover:bg-surface"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isAddingModel || !newModelName.trim()}
              className="inline-flex min-w-24 items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isAddingModel && <Loader2 size={14} className="animate-spin" />}
              Add Model
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
