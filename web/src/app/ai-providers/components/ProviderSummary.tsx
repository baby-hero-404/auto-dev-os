import { Cpu, KeyRound, Server } from "lucide-react";
import { PROVIDERS } from "@/lib/model-options";
import { CLI_PROFILES } from "@/lib/cliProfiles";

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
  activeTab: "api" | "cli";
}

export function ProviderSummary({
  configuredProviderCount,
  totalCredentialCount,
  activeCredentialCount,
  activeTab,
}: ProviderSummaryProps) {
  const apiProviders = PROVIDERS.filter((p) => p !== "gateway" && !p.startsWith("cli:"));
  const cliProviders = PROVIDERS.filter((p) => p.startsWith("cli:"));

  const getProviderLabel = (p: string) => {
    if (p.startsWith("cli:")) {
      const ref = Object.keys(CLI_PROFILES).find(k => CLI_PROFILES[k].credentialProvider === p);
      return ref ? CLI_PROFILES[ref].label : p;
    }
    return p === "9router" ? "9Router" : p.charAt(0).toUpperCase() + p.slice(1);
  };

  const config = {
    api: {
      totalProviders: apiProviders.length,
      providerListText: apiProviders.map(getProviderLabel).join(", "),
      credentialLabel: "Credential pool",
      credentialEntity: "key",
      readinessLabel: "Gateway readiness",
      readinessDetail: "Agents route through the gateway pool",
    },
    cli: {
      totalProviders: cliProviders.length,
      providerListText: cliProviders.map(getProviderLabel).join(", "),
      credentialLabel: "Auth profiles",
      credentialEntity: "profile",
      readinessLabel: "Execution readiness",
      readinessDetail: "Sandboxed CLIs ready for execution",
    },
  }[activeTab];

  return (
    <div className="grid gap-3 md:grid-cols-3">
      <SummaryCard
        icon={<Server size={18} />}
        label="Configured providers"
        value={`${configuredProviderCount}/${config.totalProviders}`}
        detail={config.providerListText}
      />
      <SummaryCard
        icon={<KeyRound size={18} />}
        label={config.credentialLabel}
        value={`${totalCredentialCount}`}
        detail={`${activeCredentialCount} active ${config.credentialEntity}${activeCredentialCount === 1 ? "" : "s"}`}
      />
      <SummaryCard
        icon={<Cpu size={18} />}
        label={config.readinessLabel}
        value={activeCredentialCount > 0 ? "Ready" : "Waiting"}
        detail={config.readinessDetail}
        tone={activeCredentialCount > 0 ? "success" : "muted"}
      />
    </div>
  );
}
