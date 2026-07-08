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
  default_model_level?: string;
  default_autonomy?: string;
  auto_review_policy?: string;
  max_retries?: number;
  max_review_fix_cycles?: number;
  default_branch?: string;
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
  display_name: string;
  url: string;
  provider: string;
  branch: string;
  clone_path: string;
  clone_status: string;
  last_validated_at?: string;
  git_account_id?: string;
  token?: string;
};

export type TaskStatus =
  | "todo"
  | "context_loading"
  | "analyzing"
  | "spec_review"
  | "coding"
  | "reviewing"
  | "fixing"
  | "testing"
  | "pr_ready"
  | "human_review"
  | "merged"
  | "failed";

export type Task = {
  id: string;
  project_id: string;
  agent_id?: string;
  parent_task_id?: string;
  repository_id?: string;
  title: string;
  description: string;
  status: TaskStatus;
  complexity: "easy" | "medium" | "hard";
  priority: number;
  labels: string[];
  pr_urls?: string[];
  pr_metadata?: Record<string, unknown>;
  analysis?: TaskAnalysis;
  spec_status: string;
  clarifications?: ClarificationRound[];
  created_at: string;
  updated_at: string;
};

export type ClarificationRound = {
  round: number;
  timestamp: string;
  questions: string[];
  response: string;
};

export type AffectedFile = {
  repo: string;
  file: string;
  confidence: number;
  reason: string;
};

export type ComplexityDetails = {
  architecture: string;
  data_migration: boolean;
  breaking_change: boolean;
};

export type RiskDetail = {
  risk: string;
  probability: string;
  severity: string;
  owner: string;
  mitigation: string;
};

export type TaskDAG = {
  id: string;
  depends_on: string[];
  complexity?: ComplexityDetails;
};

export type ExecutionBoundary = {
  module: string;
  root: string;
  repo_name?: string;
  repository_id?: string;
  capabilities: string[];
};

export type ExpandedBoundary = {
  file: string;
  reason: string;
  capability?: string;
  risk?: string;
};

export type ExecutionPhase = {
  phase: string;
  tasks: string[];
};

export type ExecutionProfile = {
  agent: string;
  skills: string[];
};

export type ExecutionConstraints = {
  parallelizable: boolean;
  max_files: number;
  estimated_tokens: number;
  max_risk: string;
  risk_multiplier?: number;
};

export type ExecutionUnit = {
  id: string;
  objective: string;
  tasks: string[];
  execution_profile: ExecutionProfile;
  constraints: ExecutionConstraints;
  dependencies?: string[];
};

export type TaskAnalysis = {
  complexity: "easy" | "medium" | "hard";
  primary_category?: string;
  scope: string;
  affected_files: string[] | AffectedFile[];
  risks: string[];
  execution_plan: string[];
  clarification_questions?: string[];
  task_rules?: string[];
  required_skills?: string[];
  risk_domains?: string[];
  proposal_md?: string;
  specs_md?: string;
  design_md?: string;
  tasks_md?: string;
  tasks?: TaskDAG[];
  complexity_details?: ComplexityDetails;
  risks_details?: RiskDetail[];
  required_skills_map?: Record<string, string[]>;
  execution_boundaries?: ExecutionBoundary[];
  expanded_boundaries?: ExpandedBoundary[];
  execution_units?: ExecutionUnit[];
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
  credential_id?: string;
  key_label?: string;
  provider: string;
  model: string;
  level_group: string;
  requests: number;
  success_requests: number;
  failed_requests: number;
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
  model_level_group: string;
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
  model_level_group: string;
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

export type GatewayUsagePoint = {
  bucket: string;
  requests: number;
  prompt_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost_usd: number;
  avg_latency_ms: number;
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

export type RecentFailure = {
  task_id: string;
  project_id: string;
  project_name: string;
  title: string;
  failure_reason: string;
  workflow_step: string;
  failed_at: string;
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
export type ComboEntry = {
  provider: string;
  model: string;
  priority: number;
  level_group?: string;
};

export type ProviderModel = {
  id: string;
  org_id: string;
  provider: string;
  level_group: "fast" | "balanced" | "powerful";
  model_name: string;
  priority: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

export type CreateProviderModelInput = {
  provider: string;
  level_group: "fast" | "balanced" | "powerful";
  model_name: string;
  priority: number;
  is_active?: boolean;
};

export type UpdateProviderModelInput = Partial<CreateProviderModelInput>;

export type SkillSource = {
  id: string;
  url: string;
  status: string;
  error?: string;
  last_synced_at?: string;
};
