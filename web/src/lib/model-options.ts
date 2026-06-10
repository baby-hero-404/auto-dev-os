export const AGENT_ROLES = ["planner", "backend", "frontend", "reviewer", "qa"] as const;
export const AUTONOMY_LEVELS = ["autonomous", "supervised", "approval_required"] as const;
export const MODEL_ROUTES = ["fast", "balanced", "powerful", "coding-default"] as const;
export const PROVIDERS = ["gateway", "openai", "anthropic", "gemini", "9router"] as const;
export const AGENT_LEVELS = ["easy", "medium", "hard"] as const;
export const ASSIGNMENT_STRATEGIES = ["manual", "auto_join"] as const;

export const AUTO_MODEL_VALUE = "__auto__";

export const GATEWAY_MODEL_BY_LEVEL: Record<string, string> = {
  easy: "fast",
  medium: "balanced",
  hard: "powerful",
};

export const MODEL_OPTIONS_BY_PROVIDER: Record<string, string[]> = {
  gateway: ["fast", "balanced", "powerful"],
  openai: ["gpt-4o-mini", "gpt-4o"],
  anthropic: ["claude-sonnet-4-20250514", "claude-opus-4-20250514"],
  gemini: ["gemini-2.5-flash", "gemini-2.5-pro"],
  "9router": ["balanced", "fast", "powerful", "premium-coding"],
};

export const MODEL_TIER_HINTS: Record<string, string> = {
  fast: "fast",
  balanced: "balanced",
  powerful: "powerful",
  "premium-coding": "premium",
  "gpt-4o-mini": "fast",
  "gpt-4o": "powerful",
  "claude-sonnet-4-20250514": "balanced",
  "claude-opus-4-20250514": "premium",
  "gemini-2.5-flash": "fast",
  "gemini-2.5-pro": "powerful",
};

export function resolveAgentModel(provider: string, selectedModel: string, level: string) {
  if (selectedModel !== AUTO_MODEL_VALUE) return selectedModel;
  if (provider === "gateway") return GATEWAY_MODEL_BY_LEVEL[level] || "balanced";
  return MODEL_OPTIONS_BY_PROVIDER[provider]?.[0] || "default";
}
