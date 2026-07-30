import { useState } from "react";
import { X, Terminal as TerminalIcon, AlertCircle } from "lucide-react";
import { toast } from "sonner";
import { CliAuthFlow } from "./CliAuthFlow";
import { CLI_AUTH_PROVIDERS } from "./cliAuthProviders";

interface CliAuthModalProps {
  provider: string;
  token: string;
  orgID: string;
  existingLabels?: string[];
  defaultLabelOverride?: string;
  isReauth?: boolean;
  onClose: () => void;
  onSave: (provider: string, label: string, apiKey: string) => Promise<string | null>;
}

export function CliAuthModal({ provider, token, orgID, existingLabels = [], defaultLabelOverride, isReauth, onClose, onSave }: CliAuthModalProps) {
  const providerName = provider.replace("cli:", "");
  
  // Auto-generate a non-conflicting default label if not re-auth
  let defaultLabel = defaultLabelOverride || `${providerName} key`;
  if (!defaultLabelOverride) {
    let counter = 2;
    while (existingLabels.some((l) => l.toLowerCase() === defaultLabel.toLowerCase())) {
      defaultLabel = `${providerName} key ${counter}`;
      counter++;
    }
  }

  const [label, setLabel] = useState(defaultLabel);
  const [saveError, setSaveError] = useState("");
  const [isSaving, setIsSaving] = useState(false);

  const title = CLI_AUTH_PROVIDERS[provider]?.title ?? "Connect CLI";

  const isDuplicate = (value: string) =>
    existingLabels.some((l) => l.toLowerCase() === (value.trim() || defaultLabel).toLowerCase());

  const handleLabelChange = (value: string) => {
    setLabel(value);
    setSaveError(isDuplicate(value) ? `"${value.trim() || defaultLabel}" is already used for this provider. Choose a different name.` : "");
  };

  const handleExit = async (payload: Record<string, string>) => {
    const finalLabel = label.trim() || defaultLabel;
    if (isDuplicate(label)) {
      setSaveError(`"${finalLabel}" is already used for this provider. Choose a different name.`);
      return;
    }
    const apiKey = JSON.stringify(payload, null, 2);
    setIsSaving(true);
    const error = await onSave(provider, finalLabel, apiKey);
    setIsSaving(false);
    if (error) {
      setSaveError(error);
    }
  };

  const handleError = (error: string) => {
    console.error("CLI Auth Error:", error);
    toast.error(error || "CLI authentication session failed. Please try again.");
    onClose();
  };

  return (
    <div
      className="fixed inset-0 z-modal grid place-items-center bg-black/45 px-4 py-6 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      onMouseDown={onClose}
    >
      <div
        className="glass-panel animate-modal-in w-full max-w-4xl max-h-[90vh] overflow-y-auto rounded-lg p-5 shadow-2xl"
        onMouseDown={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="mb-4 flex items-center justify-between">
          <h3 className="flex items-center gap-2 font-semibold text-foreground">
            <TerminalIcon size={18} className="text-brand-primary" />
            {isReauth ? `Re-authenticate ${title.replace("Connect ", "")}` : title}
          </h3>
          <button
            type="button"
            onClick={onClose}
            className="rounded p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground"
            title="Close"
          >
            <X size={16} />
          </button>
        </div>

        {/* Label input — always visible at the top */}
        <div className="mb-4">
          <label className="mb-1.5 block text-xs font-medium text-content-muted">
            Credential name
            <span className="ml-1 text-content-muted">(optional — you can change it now or later)</span>
          </label>
          <input
            type="text"
            value={label}
            onChange={(e) => handleLabelChange(e.target.value)}
            placeholder={defaultLabel}
            disabled={isReauth}
            className={`w-full rounded-lg border px-3 py-2 text-sm text-foreground placeholder:text-content-muted focus:outline-none focus:ring-2 ${
              saveError
                ? "border-red-500 bg-red-500/5 focus:ring-red-500/30"
                : isReauth 
                  ? "border-stroke bg-surface cursor-not-allowed text-content-muted" 
                  : "border-stroke bg-background focus:ring-brand-primary/50"
            }`}
          />
          {saveError && (
            <p className="mt-1.5 flex items-center gap-1.5 text-xs text-red-500">
              <AlertCircle size={12} />
              {saveError}
            </p>
          )}
        </div>

        {/* Auth flow */}
        <div className="mb-2">
          {isSaving ? (
            <div className="flex items-center justify-center gap-2 py-8 text-sm text-content-muted">
              <span className="h-4 w-4 animate-spin rounded-full border-2 border-brand-primary border-t-transparent" />
              Saving credential…
            </div>
          ) : (
            <CliAuthFlow provider={provider} orgID={orgID} token={token} onExit={handleExit} onError={handleError} />
          )}
        </div>
      </div>
    </div>
  );
}
