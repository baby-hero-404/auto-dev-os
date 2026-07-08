import { Cpu, KeyRound, Server } from "lucide-react";

interface SummaryCardProps {
  icon: React.ReactNode;
  label: string;
  value: string;
  detail: string;
  tone?: "default" | "success" | "muted";
}

function SummaryCard({ icon, label, value, detail, tone = "default" }: SummaryCardProps) {
  return (
    <div className="rounded-lg border border-stroke bg-card p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-content-muted">{label}</p>
          <p className={`mt-2 text-2xl font-semibold ${tone === "success" ? "text-emerald-600 dark:text-emerald-300" : "text-foreground"}`}>
            {value}
          </p>
          <p className="mt-1 text-xs text-content-muted">{detail}</p>
        </div>
        <div className={`grid size-9 place-items-center rounded-lg ${tone === "success" ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-300" : "bg-brand-primary-muted text-brand-primary"}`}>
          {icon}
        </div>
      </div>
    </div>
  );
}

interface ProviderSummaryProps {
  configuredProviderCount: number;
  totalCredentialCount: number;
  activeCredentialCount: number;
}

export function ProviderSummary({
  configuredProviderCount,
  totalCredentialCount,
  activeCredentialCount,
}: ProviderSummaryProps) {
  return (
    <div className="grid gap-3 md:grid-cols-3">
      <SummaryCard
        icon={<Server size={18} />}
        label="Configured providers"
        value={`${configuredProviderCount}/4`}
        detail="OpenAI, Anthropic, Gemini, 9router"
      />
      <SummaryCard
        icon={<KeyRound size={18} />}
        label="Credential pool"
        value={`${totalCredentialCount}`}
        detail={`${activeCredentialCount} active key${activeCredentialCount === 1 ? "" : "s"}`}
      />
      <SummaryCard
        icon={<Cpu size={18} />}
        label="Gateway readiness"
        value={activeCredentialCount > 0 ? "Ready" : "Waiting"}
        detail="Agents route through the gateway pool"
        tone={activeCredentialCount > 0 ? "success" : "muted"}
      />
    </div>
  );
}
