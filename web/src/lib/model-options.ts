export const AGENT_ROLES = ["planner", "backend", "frontend", "reviewer", "qa", "security-auditor", "db-architect", "documentation-writer"] as const;
export const AUTONOMY_LEVELS = ["autonomous", "supervised", "approval_required"] as const;
export const MODEL_LEVEL_GROUPS = ["fast", "balanced", "powerful"] as const;
export const PROVIDERS = ["gateway", "openai", "anthropic", "gemini", "9router"] as const;
export const AGENT_LEVELS = ["easy", "medium", "hard"] as const;
export const ASSIGNMENT_STRATEGIES = ["manual", "auto_join"] as const;
