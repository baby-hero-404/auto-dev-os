"use client";

import { FormEvent, useMemo, useState } from "react";
import { ArrowLeft, Bot, ChevronDown, Loader2, Plus, X, Zap, Scale, Gem } from "lucide-react";
import {
  AGENT_ROLES,
  ASSIGNMENT_STRATEGIES,
  AUTONOMY_LEVELS,
  MODEL_OPTIONS_BY_PROVIDER,
  PROVIDERS,
} from "@/lib/model-options";
import type { RoleTemplate, Skill } from "@/lib/types";

export type HireAgentPayload = {
  name: string;
  role: string;
  goal: string;
  autonomy_level: string;
  model_route: string;
  assignment_strategy: string;
  skill_ids: string[];
};

type TierID = "fast" | "balanced" | "powerful" | "custom";

const ROLE_TIER_HINT: Record<string, TierID> = {
  planner: "fast",
  backend: "balanced",
  frontend: "balanced",
  reviewer: "powerful",
  qa: "balanced",
};

const TIER_OPTIONS: Array<{
  id: TierID;
  title: string;
  detail: string;
  icon: typeof Zap;
}> = [
  { id: "fast", title: "Fastest & Cheapest", detail: "gateway / fast - best for planner, QA", icon: Zap },
  { id: "balanced", title: "Smart & Balanced", detail: "gateway / balanced", icon: Scale },
  { id: "powerful", title: "Most Capable", detail: "gateway / powerful - best for reviewer, hard tasks", icon: Gem },
  { id: "custom", title: "Custom", detail: "Choose provider and model manually", icon: Bot },
];

export function HireAgentWizard({
  roleTemplates,
  skills,
  isSubmitting,
  error,
  onClose,
  onSubmit,
}: {
  roleTemplates: RoleTemplate[];
  skills: Skill[];
  isSubmitting: boolean;
  error: string;
  onClose: () => void;
  onSubmit: (payload: HireAgentPayload) => void;
}) {
  const [step, setStep] = useState<1 | 2>(1);
  const [name, setName] = useState("");
  const [role, setRole] = useState<string>(AGENT_ROLES[1]);
  const [goal, setGoal] = useState("");
  const [strategy, setStrategy] = useState<string>(ASSIGNMENT_STRATEGIES[0]);
  const [autonomy, setAutonomy] = useState<string>(AUTONOMY_LEVELS[1]);
  const [tier, setTier] = useState<TierID>(ROLE_TIER_HINT.backend);
  const [customProvider, setCustomProvider] = useState("openai");
  const [customModel, setCustomModel] = useState(MODEL_OPTIONS_BY_PROVIDER.openai[0]);
  const [skillIDs, setSkillIDs] = useState<string[]>([]);
  const [localError, setLocalError] = useState("");

  const template = useMemo(() => roleTemplates.find((item) => item.role === role), [role, roleTemplates]);
  const roleOptions = useMemo(
    () => [...new Set([...AGENT_ROLES, ...roleTemplates.map((item) => item.role)])],
    [roleTemplates],
  );
  const providerOptions = PROVIDERS.filter((provider) => provider !== "gateway");
  const modelOptions = MODEL_OPTIONS_BY_PROVIDER[customProvider] || [];

  function applyRole(nextRole: string) {
    setRole(nextRole);
    setTier(ROLE_TIER_HINT[nextRole] || "balanced");
    const nextTemplate = roleTemplates.find((item) => item.role === nextRole);
    if (nextTemplate && !goal.trim()) {
      setGoal(nextTemplate.default_goal);
    }
    if (nextTemplate) {
      const defaultTools = new Set(nextTemplate.default_tools);
      setSkillIDs(skills.filter((skill) => defaultTools.has(skill.name)).map((skill) => skill.id));
    }
  }

  function handleIdentityNext(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!name.trim()) {
      setLocalError("Agent name is required.");
      return;
    }
    if (!goal.trim()) {
      setLocalError("Goal is required.");
      return;
    }
    setLocalError("");
    setStep(2);
  }

  function submitCapability(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const modelRoute = tier === "custom" ? `${customProvider}/${customModel}` : tier;
    onSubmit({
      name: name.trim(),
      role,
      goal: goal.trim(),
      autonomy_level: autonomy,
      model_route: modelRoute,
      assignment_strategy: strategy,
      skill_ids: skillIDs,
    });
  }

  function toggleSkill(skillID: string) {
    setSkillIDs((current) => current.includes(skillID) ? current.filter((id) => id !== skillID) : [...current, skillID]);
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm">
      <div className="glass-panel relative w-full max-w-2xl rounded-lg p-5 shadow-2xl animate-modal-in">
        <div className="mb-5 flex items-center justify-between border-b border-stroke pb-4">
          <div>
            <h3 className="text-lg font-semibold text-foreground">Hire Capability Agent</h3>
            <p className="text-sm text-content-muted">Step {step} of 2 - {step === 1 ? "Identity" : "Capability"}</p>
          </div>
          <button onClick={onClose} disabled={isSubmitting} className="rounded-md p-1.5 text-content-muted transition hover:bg-surface hover:text-foreground" type="button">
            <X size={18} />
          </button>
        </div>

        {step === 1 ? (
          <form onSubmit={handleIdentityNext} className="space-y-4">
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="Name">
                <input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder="Backend Specialist"
                  className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
                  autoFocus
                />
              </Field>
              <Field label="Role">
                <Select value={role} onChange={applyRole} options={roleOptions} />
              </Field>
            </div>

            <Field label="Goal">
              <textarea
                value={goal}
                onChange={(event) => setGoal(event.target.value)}
                placeholder={template?.default_goal || "Describe this agent's responsibilities."}
                rows={4}
                className="rounded-md border border-stroke bg-background px-3 py-2 text-sm text-foreground focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
              />
            </Field>

            <div className="space-y-2">
              <div className="text-xs font-semibold uppercase tracking-wide text-content-muted">Strategy</div>
              <label className="flex items-start gap-2 rounded-md border border-stroke bg-background p-3 text-sm">
                <input type="radio" checked={strategy === "auto_join"} onChange={() => setStrategy("auto_join")} className="mt-1 accent-brand-primary" />
                <span>
                  <span className="font-semibold text-foreground">Auto-join all projects</span>
                  <span className="block text-xs text-content-muted">Inherited automatically by every project.</span>
                </span>
              </label>
              <label className="flex items-start gap-2 rounded-md border border-stroke bg-background p-3 text-sm">
                <input type="radio" checked={strategy === "manual"} onChange={() => setStrategy("manual")} className="mt-1 accent-brand-primary" />
                <span>
                  <span className="font-semibold text-foreground">Assign to specific projects</span>
                  <span className="block text-xs text-content-muted">Use project assignment dropdowns after hiring.</span>
                </span>
              </label>
            </div>

            {(localError || error) && <ErrorBox message={localError || error} />}

            <div className="flex justify-end gap-2 border-t border-stroke pt-4">
              <button type="button" onClick={onClose} className="rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-foreground hover:bg-surface">Cancel</button>
              <button type="submit" className="rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white hover:opacity-90">Next</button>
            </div>
          </form>
        ) : (
          <form onSubmit={submitCapability} className="space-y-4">
            <div className="grid gap-3 md:grid-cols-2">
              {TIER_OPTIONS.map((option) => {
                const Icon = option.icon;
                const selected = tier === option.id;
                const hinted = ROLE_TIER_HINT[role] === option.id;
                return (
                  <button
                    key={option.id}
                    type="button"
                    onClick={() => setTier(option.id)}
                    className={`rounded-lg border p-3 text-left transition ${selected ? "border-brand-primary bg-brand-primary-muted" : "border-stroke bg-background hover:bg-surface"}`}
                  >
                    <div className="flex items-center gap-2">
                      <Icon size={17} className={selected ? "text-brand-primary" : "text-content-muted"} />
                      <span className="font-semibold text-foreground">{option.title}</span>
                      {hinted && <span className="rounded-full bg-surface px-2 py-0.5 text-[10px] font-semibold text-content-muted">hint</span>}
                    </div>
                    <p className="mt-1 text-xs text-content-muted">{option.detail}</p>
                  </button>
                );
              })}
            </div>

            {tier === "custom" && (
              <div className="grid gap-4 rounded-lg border border-stroke bg-background p-3 md:grid-cols-2">
                <Field label="Provider">
                  <Select
                    value={customProvider}
                    onChange={(nextProvider) => {
                      setCustomProvider(nextProvider);
                      setCustomModel(MODEL_OPTIONS_BY_PROVIDER[nextProvider]?.[0] || "");
                    }}
                    options={providerOptions}
                  />
                </Field>
                <Field label="Model">
                  <Select value={customModel} onChange={setCustomModel} options={modelOptions} />
                </Field>
              </div>
            )}

            <Field label="Autonomy">
              <Select value={autonomy} onChange={setAutonomy} options={[...AUTONOMY_LEVELS]} />
            </Field>

            <Field label="Default Skills">
              <SkillPicker skills={skills} selectedIDs={skillIDs} onToggle={toggleSkill} />
            </Field>

            {error && <ErrorBox message={error} />}

            <div className="flex justify-between gap-2 border-t border-stroke pt-4">
              <button type="button" onClick={() => setStep(1)} disabled={isSubmitting} className="inline-flex items-center gap-2 rounded-md border border-stroke px-4 py-2 text-sm font-semibold text-foreground hover:bg-surface">
                <ArrowLeft size={15} />
                Back
              </button>
              <button type="submit" disabled={isSubmitting} className="inline-flex items-center gap-2 rounded-md bg-brand-primary px-4 py-2 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50">
                {isSubmitting ? <Loader2 size={16} className="animate-spin" /> : <Plus size={16} />}
                Hire Agent
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex min-w-0 flex-col gap-1.5">
      <span className="text-xs font-semibold uppercase tracking-wide text-content-muted">{label}</span>
      {children}
    </label>
  );
}

function Select({ value, onChange, options }: { value: string; onChange: (value: string) => void; options: string[] }) {
  return (
    <div className="relative">
      <select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="w-full appearance-none rounded-md border border-stroke bg-background py-2 pl-3 pr-10 text-sm text-foreground focus:border-brand-primary focus:outline-none focus:ring-2 focus:ring-brand-primary/20"
      >
        {options.map((option) => (
          <option key={option} value={option}>{option.replace("_", " ")}</option>
        ))}
      </select>
      <ChevronDown className="pointer-events-none absolute right-3 top-3 text-content-muted" size={14} />
    </div>
  );
}

function SkillPicker({ skills, selectedIDs, onToggle }: { skills: Skill[]; selectedIDs: string[]; onToggle: (skillID: string) => void }) {
  if (skills.length === 0) return <p className="rounded border border-stroke bg-surface px-3 py-2 text-xs italic text-content-muted">No skills created yet.</p>;
  const selected = new Set(selectedIDs);
  return (
    <div className="max-h-36 overflow-y-auto rounded-md border border-stroke bg-background p-2">
      <div className="grid gap-1.5 sm:grid-cols-2">
        {skills.map((skill) => (
          <label key={skill.id} className="flex min-w-0 items-center gap-2 rounded px-2 py-1.5 text-xs text-foreground transition hover:bg-surface">
            <input type="checkbox" checked={selected.has(skill.id)} onChange={() => onToggle(skill.id)} className="size-3.5 shrink-0 accent-brand-primary" />
            <span className="truncate font-mono">{skill.name}</span>
          </label>
        ))}
      </div>
    </div>
  );
}

function ErrorBox({ message }: { message: string }) {
  return <p className="rounded border border-red-500/20 bg-red-500/10 p-2 text-xs font-medium text-red-600 dark:text-red-400">{message}</p>;
}
