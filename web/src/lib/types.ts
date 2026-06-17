export type User = {
  id: string;
  email: string;
  org_id: string;
  role: string;
};

export type AuthResponse = {
  user: User;
  tokens: {
    access_token: string;
    refresh_token: string;
    token_type: string;
    expires_in: number;
  };
};

export type Project = {
  id: string;
  org_id: string;
  name: string;
  description: string;
  repositories_count?: number;
  agents_count?: number;
  tasks_done_count?: number;
  tasks_total_count?: number;
  created_at: string;
  updated_at: string;
};

export type Repository = {
  id: string;
  project_id: string;
  url: string;
  provider: string;
  branch: string;
  clone_path: string;
  clone_status: string;
  last_validated_at?: string;
  git_account_id?: string;
  token?: string;
};

export type Task = {
  id: string;
  project_id: string;
  agent_id?: string;
  parent_task_id?: string;
  title: string;
  description: string;
  status: string;
  complexity: "easy" | "medium" | "hard";
  priority: number;
  labels: string[];
  analysis?: TaskAnalysis;
  spec_status: string;
  created_at: string;
  updated_at: string;
};

export type TaskAnalysis = {
  complexity: "easy" | "medium" | "hard";
  scope: string;
  affected_files: string[];
  risks: string[];
  execution_plan: string[];
  clarification_questions?: string[];
  proposal_md?: string;
  specs_md?: string;
  design_md?: string;
  tasks_md?: string;
};

export type WorkflowArtifact = {
  id: string;
  job_id: string;
  step: string;
  type: "diff" | "patch" | string;
  name: string;
  payload: unknown;
  created_at: string;
};

export type WorkflowJob = {
  id: string;
  task_id: string;
  agent_id?: string;
  status: string;
  step: string;
  attempts: number;
  last_error: string;
  created_at: string;
  updated_at: string;
};

export type WorkflowCheckpoint = {
  id: string;
  task_id: string;
  job_id?: string;
  step: string;
  state: Record<string, unknown>;
  created_at: string;
};

export type WorkflowStatus = {
  task: Task;
  job?: WorkflowJob;
  checkpoints: WorkflowCheckpoint[];
};

export type TaskLog = {
  id: string;
  task_id: string;
  job_id?: string;
  level: string;
  message: string;
  created_at: string;
};

export type Skill = {
  id: string;
  name: string;
  description: string;
  schema: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type TokenUsageSummary = {
  project_id?: string;
  provider: string;
  model: string;
  tier: string;
  requests: number;
  prompt_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost_usd: number;
  avg_latency_ms: number;
};

export type Agent = {
  id: string;
  org_id?: string;
  project_id?: string;
  name: string;
  role: string;
  goal: string;
  autonomy_level: "autonomous" | "supervised" | "approval_required";
  context_config: Record<string, unknown>;
  model_route: string;
  status: string;
  assignment_strategy?: string;
  created_at: string;
  updated_at: string;
};

export type RoleTemplate = {
  id: string;
  role: string;
  default_goal: string;
  default_tools: string[];
  created_at: string;
};

export type Organization = {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
};

export type Rule = {
  id: string;
  org_id?: string;
  project_id?: string;
  scope: "global" | "project";
  content: string;
  enforcement: "strict" | "advisory";
  created_at: string;
  updated_at: string;
};

// ─── Phase 5: Analytics + Audit + PR ─────────────────────────────────

export type OverviewStats = {
  total_projects: number;
  total_tasks: number;
  active_tasks: number;
  completed_tasks: number;
  failed_tasks: number;
  running_agents: number;
  total_agents: number;
  success_rate: number;
  avg_completion_ms: number;
  open_prs: number;
  total_token_cost: number;
  total_tokens_used: number;
};

export type AgentStats = {
  agent_id: string;
  agent_name: string;
  role: string;
  model_route: string;
  status: string;
  task_count: number;
  success_count: number;
  fail_count: number;
  success_rate: number;
  retry_count: number;
  total_tokens: number;
  total_cost_usd: number;
};

export type TaskStatusDistribution = {
  status: string;
  count: number;
};

export type TaskTimeSeries = {
  bucket: string;
  created: number;
  completed: number;
  failed: number;
};

export type TaskAnalytics = {
  distribution: TaskStatusDistribution[];
  time_series: TaskTimeSeries[];
};

export type WorkflowStepStats = {
  step: string;
  avg_ms: number;
  total_runs: number;
  fail_count: number;
};

export type WorkflowAnalytics = {
  total_workflows: number;
  completed_count: number;
  failed_count: number;
  completion_rate: number;
  avg_duration_ms: number;
  step_stats: WorkflowStepStats[];
};

export type AuditLog = {
  id: string;
  org_id?: string;
  user_id?: string;
  agent_id?: string;
  task_id?: string;
  action: string;
  entity_type: string;
  entity_id: string;
  details: Record<string, unknown>;
  ip_address: string;
  created_at: string;
};

export type EpisodicMemory = {
  id: string;
  agent_id: string;
  project_id?: string;
  task_id?: string;
  session_id?: string;
  tier: "working" | "episodic" | "semantic" | "procedural";
  content: string;
  summary: string;
  category: string;
  tags: string[];
  metadata: Record<string, unknown>;
  access_count: number;
  decay_score: number;
  last_accessed: string;
  created_at: string;
  updated_at: string;
};

export type KnowledgeEdge = {
  id: string;
  source_id: string;
  target_id: string;
  relation: string;
  weight: number;
  created_at: string;
};

export type LearningSuggestion = {
  id: string;
  agent_id: string;
  project_id?: string;
  task_id?: string;
  suggestion_type: "rule" | "prompt_patch" | "skill" | "pattern";
  title: string;
  description: string;
  content: string;
  confidence: number;
  status: "pending" | "approved" | "rejected" | "applied";
  reviewed_by?: string;
  reviewed_at?: string;
  metadata: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type MemorySearchResult = {
  memory: EpisodicMemory;
  bm25_score: number;
  vector_score: number;
  graph_score: number;
  final_score: number;
};

export type GitAccount = {
  id: string;
  org_id: string;
  provider: string;
  display_name: string;
  base_url: string;
  created_at: string;
  updated_at: string;
};

export type ProviderCredential = {
  id: string;
  provider: string;
  label: string;
  base_url?: string;
  status: "active" | "rate_limited" | "disabled";
  priority: number;
  configured: boolean;
  key_suffix?: string;
  cooldown_until?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

export type CreateProviderCredentialInput = {
  provider: string;
  label: string;
  api_key: string;
  base_url?: string;
  priority?: number;
};

export type TestProviderCredentialInput = {
  provider: string;
  api_key: string;
  base_url?: string;
};

export type UpdateProviderCredentialInput = {
  label?: string;
  api_key?: string;
  base_url?: string;
  status?: ProviderCredential["status"];
  priority?: number;
};

export type VirtualKey = {
  id: string;
  name: string;
  key_prefix: string;
  project_id?: string;
  agent_id?: string;
  budget_limit_usd?: number;
  budget_used_usd: number;
  rpm_limit?: number;
  tpm_limit?: number;
  status: "active" | "exhausted" | "revoked";
  expires_at?: string;
  created_at: string;
};

export type CreatedVirtualKey = VirtualKey & {
  key: string;
};

export type CreateVirtualKeyInput = {
  name: string;
  project_id?: string;
  agent_id?: string;
  budget_limit_usd?: number;
  rpm_limit?: number;
  tpm_limit?: number;
  expires_at?: string;
};

export type UpdateVirtualKeyInput = {
  name?: string;
  budget_limit_usd?: number;
  rpm_limit?: number;
  tpm_limit?: number;
  status?: VirtualKey["status"];
  expires_at?: string;
};

export type ComboEntry = {
  provider: string;
  model: string;
  priority: number;
  tier?: string;
};

export type ModelRoute = {
  id: string;
  org_id: string;
  name: string;
  route_type: "tier" | "combo";
  config: ComboEntry[];
  is_default: boolean;
  created_at: string;
  updated_at: string;
};

export type CreateModelRouteInput = {
  name: string;
  route_type: "tier" | "combo";
  config: ComboEntry[];
  is_default?: boolean;
};

export type UpdateModelRouteInput = Partial<CreateModelRouteInput>;
