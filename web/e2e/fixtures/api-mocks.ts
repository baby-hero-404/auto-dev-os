import type { Page, Route } from "@playwright/test";

export const now = "2026-06-01T00:00:00.000Z";
export const orgID = "org-123";
export const projectID = "proj-1";
export const accessToken = "mock-access-token";

export type AgentFixture = {
  id: string;
  org_id?: string;
  project_id?: string;
  name: string;
  role: string;
  provider: string;
  model: string;
  level: number;
  status: string;
  autonomy_level?: "autonomous" | "supervised" | "approval_required";
  created_at: string;
  updated_at: string;
};

export type GitAccountFixture = {
  id: string;
  org_id: string;
  provider: string;
  display_name: string;
  base_url: string;
  created_at: string;
  updated_at: string;
};

export type AppMockState = {
  projectAgents: AgentFixture[];
  gitAccounts: GitAccountFixture[];
};

export function createMockState(overrides: Partial<AppMockState> = {}): AppMockState {
  return {
    projectAgents: [],
    gitAccounts: [],
    ...overrides,
  };
}

export function defaultAgent(overrides: Partial<AgentFixture> = {}): AgentFixture {
  return {
    id: "agent-1",
    org_id: orgID,
    name: "Hermes Bot",
    role: "backend",
    provider: "google",
    model: "gemini-1.5-pro",
    level: 3,
    status: "idle",
    autonomy_level: "supervised",
    created_at: now,
    updated_at: now,
    ...overrides,
  };
}

export function defaultGitAccount(overrides: Partial<GitAccountFixture> = {}): GitAccountFixture {
  return {
    id: "git-acc-1",
    org_id: orgID,
    provider: "github",
    display_name: "Default GitHub Dev Account",
    base_url: "",
    created_at: now,
    updated_at: now,
    ...overrides,
  };
}

export async function seedSession(page: Page) {
  await page.addInitScript(
    ({ sessionToken, sessionOrgID }) => {
      window.localStorage.setItem(
        "autocodeos.session",
        JSON.stringify({
          token: sessionToken,
          refresh_token: "mock-refresh-token",
          user: {
            id: "u-123",
            email: "test@autocodeos.com",
            org_id: sessionOrgID,
            role: "admin",
          },
        }),
      );
    },
    { sessionToken: accessToken, sessionOrgID: orgID },
  );
}

export function attachBrowserDiagnostics(page: Page, label: string) {
  page.on("console", (msg) => {
    console.log(`[BROWSER CONSOLE ${label}]: ${msg.text()}`);
  });
  page.on("pageerror", (err) => {
    console.log(`[BROWSER ERROR ${label}]: ${err.message}`);
  });
  page.on("requestfailed", (request) => {
    console.log(`[REQUEST FAILED ${label}]: ${request.url()} - ${request.failure()?.errorText}`);
  });
}

export async function installApiMocks(page: Page, state = createMockState()) {
  await page.route("**/api/v1/health", async (route) => {
    await json(route, { status: "ok", version: "0.2.0" });
  });

  await page.route("**/api/v1/auth/login", async (route) => {
    await json(route, {
      user: {
        id: "u-123",
        email: "test@autocodeos.com",
        org_id: orgID,
        role: "admin",
      },
      tokens: {
        access_token: accessToken,
        refresh_token: "mock-refresh-token",
        token_type: "Bearer",
        expires_in: 3600,
      },
    });
  });

  await page.route(`**/api/v1/organizations/${orgID}`, async (route) => {
    await json(route, { id: orgID, name: "Test Org", created_at: now, updated_at: now });
  });

  await page.route(`**/api/v1/organizations/${orgID}/projects`, async (route) => {
    if (route.request().method() === "POST") {
      await json(route, {
        id: "proj-abc",
        org_id: orgID,
        name: "New Project",
        description: "A newly created project",
        created_at: now,
        updated_at: now,
      }, 201);
      return;
    }

    await json(route, [
      {
        id: projectID,
        org_id: orgID,
        name: "Website Refactor",
        description: "Refactoring legacy website code",
        created_at: now,
        updated_at: now,
      },
    ]);
  });

  await page.route(`**/api/v1/projects/${projectID}`, async (route) => {
    await json(route, {
      id: projectID,
      org_id: orgID,
      name: "Website Refactor",
      description: "Refactoring legacy website code",
      created_at: now,
      updated_at: now,
    });
  });

  await page.route(`**/api/v1/projects/${projectID}/repositories`, async (route) => {
    await json(route, [
      {
        id: "repo-1",
        project_id: projectID,
        url: "https://github.com/test/repo.git",
        provider: "github",
        branch: "main",
        clone_path: "/tmp/repo",
        clone_status: "cloned",
      },
    ]);
  });

  await page.route(`**/api/v1/projects/${projectID}/tasks`, async (route) => {
    await json(route, [
      {
        id: "task-1",
        project_id: projectID,
        title: "Add API Authentication",
        description: "Implement JWT login and verification endpoints",
        status: "todo",
        complexity: "easy",
        priority: 1,
        labels: ["auth", "backend"],
        spec_status: "none",
        created_at: now,
        updated_at: now,
      },
    ]);
  });

  await page.route(`**/api/v1/projects/${projectID}/agents`, async (route) => {
    if (route.request().method() === "POST") {
      const payload = route.request().postDataJSON() as Partial<AgentFixture> & { agent_id?: string };
      const created = defaultAgent({
        ...payload,
        id: payload.agent_id || "agent-1",
        org_id: undefined,
        project_id: projectID,
      });
      state.projectAgents = [created];
      await json(route, created, 201);
      return;
    }

    await json(route, state.projectAgents);
  });

  await page.route("**/api/v1/skills", async (route) => {
    await json(route, [
      {
        id: "skill-1",
        name: "clean-code",
        description: "Clean code guidelines and validation rules",
        schema: {},
        created_at: now,
        updated_at: now,
      },
    ]);
  });

  await page.route("**/api/v1/skills/sources", async (route) => {
    await json(route, []);
  });

  await page.route(`**/api/v1/organizations/${orgID}/agents`, async (route) => {
    await json(route, [defaultAgent()]);
  });

  await page.route(`**/api/v1/organizations/${orgID}/rules`, async (route) => {
    await json(route, [
      {
        id: "rule-1",
        org_id: orgID,
        scope: "global",
        enforcement: "strict",
        name: "Strict Rules",
        content: "No console.logs",
        created_at: now,
        updated_at: now,
      },
    ]);
  });

  await page.route(`**/api/v1/projects/${projectID}/rules`, async (route) => {
    await json(route, []);
  });

  await page.route(`**/api/v1/organizations/${orgID}/provider-credentials`, async (route) => {
    await json(route, []);
  });

  await page.route("**/api/v1/role-templates", async (route) => {
    await json(route, []);
  });

  await page.route("**/api/v1/agents/*/skills", async (route) => {
    await json(route, []);
  });

  await page.route("**/api/v1/agents/*/memories", async (route) => {
    await json(route, []);
  });

  await page.route("**/api/v1/analytics/overview**", async (route) => {
    await json(route, {
      total_projects: 1,
      active_projects: 1,
      total_tasks: 1,
      active_tasks: 0,
      completed_tasks: 0,
      failed_tasks: 0,
      running_agents: state.projectAgents.length,
      total_agents: state.projectAgents.length,
      open_prs: 0,
      success_rate: 0,
      avg_completion_ms: 0,
      total_token_cost: 0.125,
      total_tokens_used: 1000,
    });
  });

  await page.route("**/api/v1/analytics/token-usage**", async (route) => {
    await json(route, [
      {
        project_id: "proj-1",
        credential_id: "cred-1",
        key_label: "OpenAI-Prod-Key1",
        provider: "openai",
        model: "gpt-4o",
        tier: "balanced",
        requests: 10,
        success_requests: 8,
        failed_requests: 2,
        prompt_tokens: 500,
        output_tokens: 500,
        total_tokens: 1000,
        cost_usd: 0.125,
        avg_latency_ms: 150,
      },
    ]);
  });

  await page.route("**/api/v1/analytics/gateway-usage**", async (route) => {
    await json(route, [
      {
        bucket: now,
        requests: 10,
        prompt_tokens: 500,
        output_tokens: 500,
        total_tokens: 1000,
        cost_usd: 0.125,
        avg_latency_ms: 150,
      },
    ]);
  });

  await page.route("**/api/v1/analytics/failures**", async (route) => {
    await json(route, [
      {
        task_id: "task-1",
        project_id: projectID,
        project_name: "Website Refactor",
        title: "Add API Authentication",
        failure_reason: "Mocked failure for dashboard diagnostics",
        workflow_step: "test",
        failed_at: now,
      },
    ]);
  });

  await page.route("**/api/v1/analytics/agents**", async (route) => {
    await json(route, []);
  });

  await page.route("**/api/v1/analytics/tasks**", async (route) => {
    await json(route, {
      by_status: { todo: 1, spec_review: 0, coding: 0, testing: 0, success: 0, failed: 0 },
      throughput: [],
    });
  });

  await page.route("**/api/v1/analytics/workflows**", async (route) => {
    await json(route, {
      completion_rate: 100,
      avg_duration_ms: 5000,
      total_workflows: 1,
      step_stats: [],
    });
  });

  await page.route(`**/api/v1/organizations/${orgID}/git-accounts`, async (route) => {
    if (route.request().method() === "POST") {
      const payload = route.request().postDataJSON() as {
        provider: string;
        display_name: string;
        base_url?: string;
      };
      const created = defaultGitAccount({
        id: `git-acc-${state.gitAccounts.length + 1}`,
        provider: payload.provider,
        display_name: payload.display_name,
        base_url: payload.base_url || "",
      });
      state.gitAccounts = [...state.gitAccounts, created];
      await json(route, created, 201);
      return;
    }

    await json(route, state.gitAccounts);
  });

  await page.route("**/api/v1/git-accounts/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const segments = url.pathname.split("/");
    const accountID = segments[segments.indexOf("git-accounts") + 1];

    if (request.method() === "POST" && url.pathname.endsWith("/test")) {
      await json(route, { status: "success" });
      return;
    }

    if (request.method() === "DELETE") {
      state.gitAccounts = state.gitAccounts.filter((account) => account.id !== accountID);
      await route.fulfill({ status: 204 });
      return;
    }

    await json(route, { error: "Unhandled git account route" }, 404);
  });

  await page.route("**/api/v1/tasks/**", async (route) => {
    const request = route.request();
    const url = request.url();
    const method = request.method();
    
    const segments = new URL(url).pathname.split("/");
    const taskIndex = segments.indexOf("tasks");
    const taskId = segments[taskIndex + 1];
    const subAction = segments[taskIndex + 2];

    if (method === "GET") {
      if (subAction === "workflow") {
        await json(route, {
          task: {
            id: taskId,
            project_id: projectID,
            title: "Add API Authentication",
            description: "Implement JWT login and verification endpoints",
            status: "todo",
            complexity: "easy",
            priority: 1,
            labels: ["auth", "backend"],
            spec_status: "review",
            created_at: now,
            updated_at: now,
            analysis: {
              scope: "Implement JWT auth endpoints",
              complexity_details: {
                architecture: "rest",
                data_migration: false,
                breaking_change: false,
              },
              risk_domains: ["security"],
              risks_details: [
                {
                  risk: "Token leak",
                  probability: "low",
                  severity: "high",
                  mitigation: "Use HTTPOnly cookies",
                  owner: "Hermes Bot"
                }
              ],
              execution_plan: ["Create login route", "Create verify route"],
              execution_boundaries: [
                {
                  module: "auth",
                  root: "src/auth",
                  repo_name: "auto-dev-os",
                  capabilities: ["modify_existing"]
                }
              ],
              expanded_boundaries: [],
              execution_units: [
                {
                  id: "code_backend_0",
                  objective: "Write database models",
                  tasks: ["Create user schema", "Set up indexes"],
                  execution_profile: { type: "backend" },
                  constraints: { parallelizable: false, max_files: 2, estimated_tokens: 100, max_risk: "low" }
                },
                {
                  id: "code_frontend_0",
                  objective: "Build login page",
                  tasks: ["Implement input forms", "Add form validation"],
                  execution_profile: { type: "frontend" },
                  constraints: { parallelizable: false, max_files: 2, estimated_tokens: 100, max_risk: "low" }
                }
              ]
            }
          },
          job: {
            id: "job-1",
            status: "running",
            step: "code_backend_0",
            progress: 50,
            step_index: 3,
            steps_total: 6,
            error_count: 0,
            created_at: now,
            updated_at: now,
          },
          checkpoints: [
            {
              id: "cp-1",
              task_id: taskId,
              step: "context_load",
              status: "success",
              message: "Context loaded successfully",
              created_at: now,
            },
            {
              id: "cp-2",
              task_id: taskId,
              step: "code_backend_0",
              status: "running",
              message: "Writing models...",
              created_at: now,
            }
          ]
        });
        return;
      }

      if (subAction === "logs") {
        if (segments[taskIndex + 3] === "stream") {
          await route.fulfill({
            status: 200,
            contentType: "text/event-stream",
            body: "event: log\ndata: {\"id\":\"log-sse-1\",\"task_id\":\"task-1\",\"level\":\"info\",\"message\":\"SSE log stream active\",\"timestamp\":\"2026-06-01T00:00:00.000Z\"}\n\nevent: log\ndata: {\"id\":\"log-sse-2\",\"task_id\":\"task-1\",\"level\":\"info\",\"message\":\"checkpoint: load success\",\"timestamp\":\"2026-06-01T00:00:00.050Z\"}\n\n",
          });
          return;
        }

        await json(route, [
          {
            id: "log-1",
            task_id: taskId,
            level: "info",
            message: "Analyzing workspace constraints...",
            timestamp: now,
          },
          {
            id: "log-2",
            task_id: taskId,
            level: "info",
            message: "checkpoint: load success",
            timestamp: now,
          }
        ]);
        return;
      }

      // Default task get
      await json(route, {
        id: taskId,
        project_id: projectID,
        title: "Add API Authentication",
        description: "Implement JWT login and verification endpoints",
        status: "todo",
        complexity: "easy",
        priority: 1,
        labels: ["auth", "backend"],
        spec_status: "review",
        created_at: now,
        updated_at: now,
        analysis: {
          scope: "Implement JWT auth endpoints",
          complexity_details: {
            architecture: "rest",
            data_migration: false,
            breaking_change: false,
          },
          risk_domains: ["security"],
          risks_details: [
            {
              risk: "Token leak",
              probability: "low",
              severity: "high",
              mitigation: "Use HTTPOnly cookies",
              owner: "Hermes Bot"
            }
          ],
          execution_plan: ["Create login route", "Create verify route"],
          execution_boundaries: [
            {
              module: "auth",
              root: "src/auth",
              repo_name: "auto-dev-os",
              capabilities: ["modify_existing"]
            }
          ],
          expanded_boundaries: []
        }
      });
      return;
    }

    if (method === "POST" || method === "PATCH") {
      await json(route, {
        status: "success",
        id: taskId,
      });
      return;
    }

    await route.continue();
  });

  await page.route("**/api/v1/workflows/*/artifacts", async (route) => {
    await json(route, []);
  });
}

async function json(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}
