import { Plus, Trash2 } from "lucide-react";
import { PROVIDERS } from "@/lib/model-options";
import type { ProviderModel } from "@/lib/types";

interface ModelRoutingRulesProps {
  selectedProvider: string;
  setSelectedProvider: (provider: string) => void;
  providerModels: ProviderModel[];
  credentialsByProvider: Record<string, unknown[]>;
  onAdjustPriority: (modelID: string, currentPriority: number, amount: number) => void;
  onToggleActive: (modelID: string, currentActive: boolean) => void;
  onDeleteModel: (modelID: string) => void;
  onOpenAddModel: (level: "fast" | "balanced" | "powerful") => void;
}

export function ModelRoutingRules({
  selectedProvider,
  setSelectedProvider,
  providerModels,
  credentialsByProvider,
  onAdjustPriority,
  onToggleActive,
  onDeleteModel,
  onOpenAddModel,
}: ModelRoutingRulesProps) {
  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-1">
        <h3 className="text-lg font-semibold text-foreground">Model Routing Rules</h3>
        <p className="text-sm text-content-muted">
          Configure specific routing groups (Fast, Balanced, Powerful) for each provider.
        </p>
      </div>

      {/* Provider Tabs */}
      <div className="flex gap-2 border-b border-stroke pb-px">
        {PROVIDERS.filter((provider) => provider !== "gateway").map((provider) => {
          const count = credentialsByProvider[provider]?.length || 0;
          const active = selectedProvider === provider;
          return (
            <button
              key={provider}
              type="button"
              onClick={() => setSelectedProvider(provider)}
              className={`flex items-center gap-2 border-b-2 px-4 py-2.5 text-sm font-semibold capitalize transition-all cursor-pointer ${
                active
                  ? "border-brand-primary text-brand-primary"
                  : "border-transparent text-content-muted hover:text-foreground"
              }`}
            >
              {provider}
              {count > 0 && (
                <span className={`rounded-full px-1.5 py-0.5 text-[10px] font-bold ${
                  active ? "bg-brand-primary-muted text-brand-primary" : "bg-surface text-content-muted"
                }`}>
                  {count} key{count === 1 ? "" : "s"}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* Stacked Level Sections */}
      <div className="grid gap-6">
        {(["fast", "balanced", "powerful"] as const).map((level) => {
          const levelModels = providerModels.filter(
            (m) => m.provider === selectedProvider && m.level_group === level
          );

          // Sort by priority ascending
          levelModels.sort((a, b) => a.priority - b.priority);

          return (
            <div key={level} className="glass-panel overflow-hidden rounded-lg p-5">
              <div className="mb-4 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="text-base font-semibold capitalize text-foreground flex items-center gap-1.5">
                    {level === "fast" && "⚡ Fast Models"}
                    {level === "balanced" && "⚖️ Balanced Models"}
                    {level === "powerful" && "🚀 Powerful Models"}
                  </span>
                  <span className="rounded-full bg-surface px-2 py-0.5 text-xs font-semibold text-content-muted border border-stroke">
                    {levelModels.length} model{levelModels.length === 1 ? "" : "s"}
                  </span>
                </div>
                <button
                  type="button"
                  onClick={() => onOpenAddModel(level)}
                  className="inline-flex items-center gap-1 rounded bg-brand-primary px-3 py-1.5 text-xs font-semibold text-white transition hover:opacity-90 cursor-pointer"
                >
                  <Plus size={12} />
                  Add Model
                </button>
              </div>

              {levelModels.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-6 text-center border border-dashed border-stroke rounded-lg bg-surface/25">
                  <p className="text-sm text-content-muted">
                    No {level} models configured yet. Add one to enable high-speed tasks.
                  </p>
                </div>
              ) : (
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-sm">
                    <thead className="border-b border-stroke text-[10px] uppercase tracking-wide text-content-muted">
                      <tr>
                        <th className="px-4 py-2 w-[40%]">Model Name</th>
                        <th className="px-4 py-2 w-[20%]">Priority</th>
                        <th className="px-4 py-2 w-[20%]">Status</th>
                        <th className="px-4 py-2 w-[20%] text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody>
                      {levelModels.map((model) => (
                        <tr key={model.id} className="border-b border-stroke/50 last:border-b-0 align-middle hover:bg-surface/10">
                          <td className="px-4 py-3 font-medium text-foreground font-mono text-xs">
                            {model.model_name}
                          </td>
                          <td className="px-4 py-3">
                            <div className="flex items-center gap-2">
                              <span className="inline-flex items-center rounded bg-surface px-2 py-0.5 text-xs font-semibold text-content-muted border border-stroke">
                                P{model.priority}
                              </span>
                              <div className="flex flex-col">
                                <button
                                  type="button"
                                  onClick={() => onAdjustPriority(model.id, model.priority, -1)}
                                  disabled={model.priority === 0}
                                  className="text-[10px] text-content-muted hover:text-foreground cursor-pointer disabled:opacity-30 disabled:cursor-not-allowed"
                                  title="Increase priority"
                                >
                                  ▲
                                </button>
                                <button
                                  type="button"
                                  onClick={() => onAdjustPriority(model.id, model.priority, 1)}
                                  className="text-[10px] text-content-muted hover:text-foreground cursor-pointer"
                                  title="Decrease priority"
                                >
                                  ▼
                                </button>
                              </div>
                            </div>
                          </td>
                          <td className="px-4 py-3">
                            <button
                              type="button"
                              onClick={() => onToggleActive(model.id, model.is_active)}
                              className={`inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-semibold transition-all cursor-pointer ${
                                model.is_active
                                  ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300 border-emerald-500/20"
                                  : "bg-surface text-content-muted border-stroke"
                              }`}
                            >
                              <span className={`size-1.5 rounded-full ${model.is_active ? "bg-emerald-500" : "bg-content-muted"}`} />
                              {model.is_active ? "Active" : "Inactive"}
                            </button>
                          </td>
                          <td className="px-4 py-3 text-right">
                            <button
                              type="button"
                              onClick={() => onDeleteModel(model.id)}
                              className="rounded p-1 text-content-muted transition-colors hover:bg-danger/10 hover:text-danger cursor-pointer"
                              title="Delete model"
                            >
                              <Trash2 size={13} />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </section>
  );
}
