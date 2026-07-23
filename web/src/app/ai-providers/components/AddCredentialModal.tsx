import { FormEvent, useEffect, useState } from "react";
import { CheckCircle2, ChevronDown, Eye, EyeOff, KeyRound, Loader2, Plus, Terminal as TerminalIcon, X, XCircle } from "lucide-react";
import { PROVIDERS } from "@/lib/model-options";
import { api } from "@/lib/api";
import dynamic from "next/dynamic";

const InteractiveTerminal = dynamic(
  () => import("./InteractiveTerminal").then((mod) => mod.InteractiveTerminal),
  { ssr: false }
);

const BASE_URL_PLACEHOLDERS: Record<string, string> = {
  openai: "https://api.openai.com/v1 (optional)",
  anthropic: "https://api.anthropic.com (optional)",
  gemini: "https://generativelanguage.googleapis.com (optional)",
  "9router": "https://api.9router.com/v1 (optional)",
};

type FormState = {
  provider: string;
  label: string;
  apiKey: string;
  baseURL: string;
  priority: string;
};

export function generatedCredentialLabel(provider: string, apiKey = "") {
  const cleanKey = apiKey.trim();
  if (cleanKey.length > 4) {
    const suffix = cleanKey.slice(-4);
    return `${provider} key ${suffix}`;
  }
  return `${provider} key`;
}

function testKeyButtonClass(state: "idle" | "testing" | "success" | "error") {
  switch (state) {
    case "success":
      return "border-emerald-500/30 bg-emerald-500/10 text-emerald-700 hover:bg-emerald-500/20 dark:text-emerald-300";
    case "error":
      return "border-red-500/30 bg-red-500/10 text-red-700 hover:bg-red-500/20 dark:text-red-300";
    case "testing":
      return "border-brand-primary/30 bg-brand-primary-muted text-brand-primary";
    default:
      return "border-stroke text-foreground hover:bg-surface";
  }
}

function FlowStep({
  index,
  label,
  active,
  pending = false,
}: {
  index: string;
  label: string;
  active: boolean;
  pending?: boolean;
}) {
  return (
    <div className="flex items-center justify-center gap-2 border-r border-stroke px-2 py-2 last:border-r-0">
      <span
        className={`grid size-5 place-items-center rounded-full text-[10px] font-bold ${active
            ? "bg-emerald-500 text-white"
            : pending
              ? "bg-brand-primary-muted text-brand-primary"
              : "bg-background text-content-muted"
          }`}
      >
        {pending ? <Loader2 size={11} className="animate-spin" /> : active ? <CheckCircle2 size={11} /> : index}
      </span>
      <span className={active ? "font-semibold text-foreground" : "font-medium text-content-muted"}>{label}</span>
    </div>
  );
}

interface AddCredentialModalProps {
  form: FormState;
  formError: string;
  isSubmitting: boolean;
  saveState: "idle" | "saved";
  draftTestState: "idle" | "testing" | "success" | "error";
  showApiKey: boolean;
  token: string;
  orgID: string;
  onClose: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onSetForm: React.Dispatch<React.SetStateAction<FormState>>;
  onTestKey: () => void;
  onToggleApiKey: () => void;
  mode: "api" | "cli";
}

export function AddCredentialModal({
  form,
  formError,
  isSubmitting,
  saveState,
  draftTestState,
  showApiKey,
  token,
  orgID,
  onClose,
  onSubmit,
  onSetForm,
  onTestKey,
  onToggleApiKey,
  mode,
}: AddCredentialModalProps) {
  const [showBaseUrl, setShowBaseUrl] = useState(false);
  const [showTerminal, setShowTerminal] = useState(false);
  const [terminalError, setTerminalError] = useState("");
  const [wsUrl, setWsUrl] = useState("");

  async function openTerminal() {
    if (!orgID || !token) return;
    setTerminalError("");
    try {
      const { ticket } = await api.mintCliAuthWSTicket(orgID, token, form.provider);
      const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:32080/api/v1";
      const wsBaseUrl = apiUrl.replace(/^http/, "ws");
      setWsUrl(`${wsBaseUrl}/organizations/${orgID}/cli-auth/terminal?provider=${form.provider}&ticket=${ticket}`);
      setShowTerminal(true);
    } catch {
      setTerminalError("Failed to start terminal session. Please try again.");
    }
  }

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && !isSubmitting) {
        onClose();
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, isSubmitting]);

  return (
    <div
      className="fixed inset-0 z-modal grid place-items-center bg-black/45 px-4 py-6 backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      aria-labelledby="add-credential-title"
      onMouseDown={onClose}
    >
      <div
        className="glass-panel animate-modal-in w-full max-w-md rounded-lg p-5 shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <div className="mb-4 flex items-center justify-between">
          <h3 id="add-credential-title" className="flex items-center gap-2 font-semibold text-foreground">
            <Plus size={18} className="text-brand-primary" />
            {mode === "api" ? "Add API Key" : "Add CLI Auth Profile"}
          </h3>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="rounded p-1.5 text-content-muted transition-colors hover:bg-surface hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
            title="Close"
          >
            <X size={16} />
          </button>
        </div>

        {formError && (
          <div className="mb-4 rounded-md border border-red-500/25 bg-red-500/10 p-3 text-xs font-medium text-red-600 dark:text-red-300">
            {formError}
          </div>
        )}

        {!form.provider.startsWith("cli:") && (
          <div className="mb-4 grid grid-cols-3 overflow-hidden rounded-md border border-stroke bg-surface/40 text-xs">
            <FlowStep index="1" label="Enter key" active={form.apiKey.trim().length > 0} />
            <FlowStep index="2" label="Test" active={draftTestState === "success"} pending={draftTestState === "testing"} />
            <FlowStep index="3" label="Save" active={saveState === "saved"} />
          </div>
        )}

        <form onSubmit={onSubmit} className="space-y-4">
          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Provider</span>
            <div className="relative">
              <select
                value={form.provider}
                onChange={(event) => {
                  const provider = event.target.value;
                  onSetForm((prev) => {
                    const currentGen = generatedCredentialLabel(prev.provider, prev.apiKey);
                    const isAutogenerated = !prev.label.trim() || prev.label === currentGen || prev.label === `${prev.provider} key`;
                    return {
                      ...prev,
                      provider,
                      label: isAutogenerated ? generatedCredentialLabel(provider, prev.apiKey) : prev.label,
                    };
                  });
                }}
                className="w-full appearance-none rounded-md border border-stroke bg-background pl-3 pr-10 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              >
                {mode === "api" && (
                  <optgroup label="API Providers">
                    {PROVIDERS.filter((p) => p !== "gateway" && !p.startsWith("cli:")).map((provider) => (
                      <option key={provider} value={provider}>
                        {provider}
                      </option>
                    ))}
                  </optgroup>
                )}
                {mode === "cli" && (
                  <optgroup label="CLI Tools">
                    {PROVIDERS.filter((p) => p !== "gateway" && p.startsWith("cli:")).map((provider) => (
                      <option key={provider} value={provider}>
                        {provider}
                      </option>
                    ))}
                  </optgroup>
                )}
              </select>
              <ChevronDown className="absolute right-3 top-3 text-content-muted pointer-events-none" size={14} />
            </div>
          </label>

          <label className="flex flex-col gap-1.5">
            <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Label</span>
            <input
              value={form.label}
              onChange={(event) => onSetForm((prev) => ({ ...prev, label: event.target.value }))}
              placeholder="primary, backup, team-api"
              className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
            />
          </label>

          {form.provider.startsWith("cli:") ? (
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">CLI Session Configuration</span>
              <p className="text-xs text-content-muted mb-2">
                Launch the interactive terminal to run the login command (e.g. <code>claude login</code>). The session files will be automatically captured when you type <code>exit</code>.
              </p>

              {!showTerminal && (
                <div className="mb-2">
                  <button
                    type="button"
                    onClick={openTerminal}
                    className="w-full inline-flex items-center justify-center gap-2 rounded-md border border-brand-primary/30 bg-brand-primary/10 px-4 py-4 text-sm font-semibold text-brand-primary transition hover:bg-brand-primary/20"
                  >
                    <TerminalIcon size={18} />
                    Launch Interactive Login Terminal
                  </button>
                </div>
              )}

              {showTerminal && wsUrl && (
                <div className="mb-2">
                  <InteractiveTerminal
                    wsUrl={wsUrl}
                    onExit={(payload) => {
                      onSetForm((prev) => ({ ...prev, apiKey: JSON.stringify(payload, null, 2) }));
                      setShowTerminal(false);
                      setWsUrl("");
                    }}
                    onError={(err) => setTerminalError(err)}
                  />
                  {terminalError && <p className="text-red-500 text-xs mt-1">{terminalError}</p>}
                  <button
                    type="button"
                    onClick={() => {
                      setShowTerminal(false);
                      setWsUrl("");
                    }}
                    className="mt-2 text-xs text-content-muted hover:text-foreground underline"
                  >
                    Close Terminal (Cancel)
                  </button>
                </div>
              )}

              <div className="mt-2 border-t border-stroke pt-3">
                <span className="text-xs font-semibold uppercase tracking-wider text-content-muted block mb-2">Or Provide JSON Manually</span>
                <div className="relative">
                  <KeyRound className="absolute left-3 top-2.5 text-content-muted" size={14} />
                  <textarea
                    value={form.apiKey}
                    onChange={(event) => {
                      const key = event.target.value;
                      onSetForm((prev) => ({ ...prev, apiKey: key }));
                    }}
                    placeholder={'{\n  "token": "..."\n}'}
                    rows={4}
                    className="w-full rounded-md border border-stroke bg-background py-2 pl-9 pr-3 text-xs font-mono text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20 resize-none"
                  />
                </div>
              </div>
            </label>
          ) : (
            <label className="flex flex-col gap-1.5">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">API Key</span>
              <div className="relative">
                <KeyRound className="absolute left-3 top-2.5 text-content-muted" size={14} />
                <input
                  type={showApiKey ? "text" : "password"}
                  value={form.apiKey}
                  onChange={(event) => {
                    const key = event.target.value;
                    onSetForm((prev) => {
                      const currentGen = generatedCredentialLabel(prev.provider, prev.apiKey);
                      const isAutogenerated = !prev.label.trim() || prev.label === currentGen || prev.label === `${prev.provider} key`;
                      return {
                        ...prev,
                        apiKey: key,
                        label: isAutogenerated ? generatedCredentialLabel(prev.provider, key) : prev.label,
                      };
                    });
                  }}
                  placeholder="sk-..."
                  className="w-full rounded-md border border-stroke bg-background py-2 pl-9 pr-10 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
                />
                <button
                  type="button"
                  onClick={onToggleApiKey}
                  className="absolute right-3 top-2.5 text-content-muted transition-colors duration-150 hover:text-foreground"
                  title={showApiKey ? "Hide API Key" : "Show API Key"}
                >
                  {showApiKey ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </label>
          )}

          {!form.provider.startsWith("cli:") && (
            <div className="flex items-center gap-2 py-0.5">
              <input
                type="checkbox"
                id="toggle-base-url"
                checked={showBaseUrl}
                onChange={(e) => {
                  setShowBaseUrl(e.target.checked);
                  if (!e.target.checked) {
                    onSetForm((prev) => ({ ...prev, baseURL: "" }));
                  }
                }}
                className="size-3.5 cursor-pointer rounded border-stroke bg-background text-brand-primary focus:ring-0 focus:ring-offset-0"
              />
              <label htmlFor="toggle-base-url" className="text-xs font-medium text-content-muted cursor-pointer select-none hover:text-foreground transition-colors">
                Use custom Base URL
              </label>
            </div>
          )}

          {!form.provider.startsWith("cli:") && showBaseUrl && (
            <label className="flex flex-col gap-1.5 animate-fade-in">
              <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Base URL</span>
              <input
                value={form.baseURL}
                onChange={(event) => onSetForm((prev) => ({ ...prev, baseURL: event.target.value }))}
                placeholder={BASE_URL_PLACEHOLDERS[form.provider] || "https://api.openai.com/v1 (optional)"}
                className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              />
            </label>
          )}

          {!form.provider.startsWith("cli:") && (
            <label className="flex flex-col gap-1.5">
              <div className="flex items-center justify-between gap-3">
                <span className="text-xs font-semibold uppercase tracking-wider text-content-muted">Priority</span>
                <span className="text-right text-[10px] font-medium text-content-muted">Lower = runs first (0 = highest)</span>
              </div>
              <input
                type="number"
                value={form.priority}
                onChange={(event) => onSetForm((prev) => ({ ...prev, priority: event.target.value }))}
                className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground transition-all duration-150 focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              />
            </label>
          )}

          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="rounded-md border border-stroke px-4 py-2.5 text-sm font-semibold text-foreground transition hover:bg-surface disabled:cursor-not-allowed disabled:opacity-50"
            >
              Cancel
            </button>
            {!form.provider.startsWith("cli:") && (
              <button
                type="button"
                onClick={onTestKey}
                disabled={draftTestState === "testing" || isSubmitting || !form.apiKey.trim() || !token || !orgID}
                className={`inline-flex min-w-36 items-center justify-center gap-2 rounded-md border px-4 py-2.5 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-50 ${testKeyButtonClass(draftTestState)}`}
              >
                {draftTestState === "testing" && <Loader2 size={14} className="animate-spin" />}
                {draftTestState === "success" && <CheckCircle2 size={14} />}
                {draftTestState === "error" && <XCircle size={14} />}
                {draftTestState === "testing" ? "Testing" : draftTestState === "success" ? "Connected" : draftTestState === "error" ? "Retry test" : "Test connection"}
              </button>
            )}
            <button
              disabled={isSubmitting || (draftTestState !== "success" && !form.provider.startsWith("cli:")) || !token || !orgID}
              className="inline-flex min-w-32 items-center justify-center gap-2 rounded-md bg-brand-primary px-4 py-2.5 text-sm font-semibold text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
              type="submit"
            >
              {isSubmitting && <Loader2 size={14} className="animate-spin" />}
              {saveState === "saved" && !isSubmitting ? (
                <>
                  <CheckCircle2 size={14} />
                  Saved
                </>
              ) : (
                "Save"
              )}
            </button>
          </div>
          {draftTestState !== "success" && !form.provider.startsWith("cli:") && (
            <p className="text-right text-xs text-content-muted">Save unlocks after a successful connection test.</p>
          )}
        </form>
      </div>
    </div>
  );
}
