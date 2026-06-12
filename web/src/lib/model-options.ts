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
  openai: [
    "gpt-5.5",
    "gpt-5.4",
    "gpt-5.4-mini",
    "gpt-5.4-nano",
    "gpt-4o",
    "gpt-4o-mini"
  ],
  anthropic: [
    "claude-fable-5",
    "claude-opus-4-8",
    "claude-sonnet-4-6",
    "claude-haiku-4-5",
    "claude-sonnet-4-20250514",
    "claude-opus-4-20250514"
  ],
  gemini: [
    "gemini-3.1-pro-preview",
    "gemini-3.1-pro-preview-customtools",
    "gemini-3.5-flash",
    "gemini-3-flash-preview",
    "gemini-3.1-flash-lite",
    "gemini-2.5-pro",
    "gemini-2.5-flash"
  ],
  "9router": ["balanced", "fast", "powerful", "premium-coding"],
};

export const MODEL_TIER_HINTS: Record<string, string> = {
  // Gateway routes
  fast: "fast",
  balanced: "balanced",
  powerful: "powerful",
  "premium-coding": "premium",

  // OpenAI
  "gpt-5.5": "powerful (reasoning)",
  "gpt-5.4": "balanced (coding)",
  "gpt-5.4-mini": "balanced",
  "gpt-5.4-nano": "fast",
  "gpt-4o": "powerful",
  "gpt-4o-mini": "fast",

  // Anthropic
  "claude-fable-5": "powerful (reasoning)",
  "claude-opus-4-8": "powerful (coding)",
  "claude-sonnet-4-6": "balanced",
  "claude-haiku-4-5": "fast",
  "claude-sonnet-4-20250514": "balanced",
  "claude-opus-4-20250514": "premium",

  // Gemini
  "gemini-3.1-pro-preview": "powerful (agentic coding)",
  "gemini-3.1-pro-preview-customtools": "powerful (tools)",
  "gemini-3.5-flash": "balanced (fast loop)",
  "gemini-3-flash-preview": "balanced",
  "gemini-3.1-flash-lite": "fast",
  "gemini-2.5-pro": "powerful (reasoning)",
  "gemini-2.5-flash": "fast",
};

export function resolveAgentModel(provider: string, selectedModel: string, level: string) {
  if (selectedModel !== AUTO_MODEL_VALUE) return selectedModel;
  if (provider === "gateway") return GATEWAY_MODEL_BY_LEVEL[level] || "balanced";
  return MODEL_OPTIONS_BY_PROVIDER[provider]?.[0] || "default";
}
