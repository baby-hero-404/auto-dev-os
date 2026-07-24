import { useState } from "react";
import dynamic from "next/dynamic";
import { CheckCircle2, Copy } from "lucide-react";
import { toast } from "sonner";
import { CLI_AUTH_PROVIDERS } from "./cliAuthProviders";

const InteractiveTerminal = dynamic(
  () => import("./InteractiveTerminal").then((mod) => mod.InteractiveTerminal),
  { ssr: false }
);

interface CliAuthFlowProps {
  provider: string;
  orgID: string;
  token: string;
  onExit: (payload: Record<string, string>) => void;
  onError: (error: string) => void;
}

export function CliAuthFlow({ provider, orgID, token, onExit, onError }: CliAuthFlowProps) {
  const config = CLI_AUTH_PROVIDERS[provider];
  const [authUrl, setAuthUrl] = useState("");
  const [authCode, setAuthCode] = useState("");
  const [authSuccess, setAuthSuccess] = useState(false);

  if (!config) {
    onError(`Unsupported CLI provider: ${provider}`);
    return null;
  }

  return (
    <div className="mb-2">
      <div className="mb-3 text-xs text-content-muted">{config.description}</div>

      {config.preAuthWarning && !authSuccess && !authUrl && (
        <div className="mb-4 rounded-md bg-red-500/10 p-3 text-center text-sm font-medium text-red-600 dark:text-red-400">
          {config.preAuthWarning}
        </div>
      )}

      {authSuccess ? (
        <div className="mb-4 flex items-center justify-center gap-3 rounded-md border border-green-500/30 bg-green-500/10 p-4 text-center animate-fade-in">
          <CheckCircle2 className="text-green-500" size={24} />
          <p className="text-sm text-green-500 font-semibold">Authentication Successful! Finalizing setup...</p>
        </div>
      ) : authUrl ? (
        <div className="mb-4 rounded-md border border-brand-primary/30 bg-brand-primary/10 p-4 text-center animate-fade-in">
          <p className="mb-2 text-sm text-foreground font-semibold">Please authenticate to continue</p>
          <div className="flex flex-col items-center gap-3">
            <a href={authUrl} target="_blank" rel="noreferrer" className="inline-block rounded bg-brand-primary px-6 py-2.5 text-white font-bold transition-opacity hover:opacity-90 shadow-[0_0_15px_rgba(34,197,94,0.15)]">
              Click here to authenticate
            </a>
            {authCode && (
              <div className="flex items-center gap-2 rounded bg-surface px-4 py-2 border border-stroke">
                <span className="text-sm text-content-muted">Device Code:</span>
                <code className="text-brand-primary font-bold">{authCode}</code>
                <button
                  type="button"
                  onClick={() => {
                    navigator.clipboard.writeText(authCode);
                    toast.success("Code copied to clipboard");
                  }}
                  className="ml-2 rounded p-1 text-content-muted transition hover:bg-surface-hover hover:text-foreground"
                  title="Copy code"
                >
                  <Copy size={16} />
                </button>
              </div>
            )}
          </div>
          {config.steps.length > 0 && (
            <div className="mt-4 rounded bg-background/50 p-3 text-left border border-border/50">
              {config.steps.map((step, i) => (
                <p key={step} className={`text-sm text-foreground font-medium flex items-center gap-2 ${i > 0 ? "mt-2" : ""}`}>
                  <span className="flex h-5 w-5 items-center justify-center rounded-full bg-brand-primary text-xs text-white">{i + 1}</span>
                  {step}
                </p>
              ))}
            </div>
          )}
          {config.footerNote && <p className="mt-3 text-xs text-brand-primary">{config.footerNote}</p>}
        </div>
      ) : null}

      <InteractiveTerminal
        orgID={orgID}
        token={token}
        provider={provider}
        initialCommand={config.initialCommand}
        extractUrl={config.extractUrl}
        extractCode={config.extractCode}
        isSuccess={config.isSuccess}
        onExit={onExit}
        onError={onError}
        onUrlFound={setAuthUrl}
        onCodeFound={setAuthCode}
        onSuccessFound={() => setAuthSuccess(true)}
      />
    </div>
  );
}
