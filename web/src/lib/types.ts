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
  project_id: string;
  name: string;
  role: string;
  provider: string;
  model: string;
  level: number;
  status: string;
  created_at: string;
  updated_at: string;
};

export type Organization = {
  id: string;
  name: string;
  created_at: string;
  updated_at: string;
};

export type Rule = {
  id: string;
  project_id?: string;
  scope: "global" | "project";
  content: string;
  enforcement: "strict" | "advisory";
  created_at: string;
  updated_at: string;
};
