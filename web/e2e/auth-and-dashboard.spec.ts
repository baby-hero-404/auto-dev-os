import { test, expect } from "@playwright/test";

interface AgentFixture {
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
}

test.describe("Auto Code OS E2E Flows", () => {
  let projectAgents: AgentFixture[] = [];

  test.beforeEach(async ({ page }) => {
    projectAgents = [];
    page.on("console", (msg) => {
      console.log(`[BROWSER CONSOLE auth]: ${msg.text()}`);
    });
    page.on("pageerror", (err) => {
      console.log(`[BROWSER ERROR auth]: ${err.message}`);
    });
    page.on("requestfailed", (request) => {
      console.log(`[REQUEST FAILED auth]: ${request.url()} - ${request.failure()?.errorText}`);
    });
    page.on("request", (req) => {
      console.log(`[REQUEST auth]: ${req.method()} - ${req.url()}`);
    });

    // Mock health check
    await page.route("**/api/v1/health", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "ok", version: "0.2.0" }),
      });
    });

    // Mock login endpoint
    await page.route("**/api/v1/auth/login", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          user: {
            id: "u-123",
            email: "test@autocodeos.com",
            org_id: "org-123",
            role: "admin",
          },
          tokens: {
            access_token: "mock-access-token",
            refresh_token: "mock-refresh-token",
            token_type: "Bearer",
            expires_in: 3600,
          },
        }),
      });
    });

    // Mock organization details
    await page.route("**/api/v1/organizations/org-123", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "org-123",
          name: "Test Org",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }),
      });
    });

    // Mock projects list
    await page.route("**/api/v1/organizations/org-123/projects", async (route) => {
      if (route.request().method() === "POST") {
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify({
            id: "proj-abc",
            org_id: "org-123",
            name: "New Project",
            description: "A newly created project",
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          }),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify([
            {
              id: "proj-1",
              org_id: "org-123",
              name: "Website Refactor",
              description: "Refactoring legacy website code",
              created_at: new Date().toISOString(),
              updated_at: new Date().toISOString(),
            },
          ]),
        });
      }
    });

    // Mock individual project
    await page.route("**/api/v1/projects/proj-1", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "proj-1",
          org_id: "org-123",
          name: "Website Refactor",
          description: "Refactoring legacy website code",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        }),
      });
    });

    // Mock repositories list
    await page.route("**/api/v1/projects/proj-1/repositories", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "repo-1",
            project_id: "proj-1",
            url: "https://github.com/test/repo.git",
            provider: "github",
            branch: "main",
            clone_path: "/tmp/repo",
            clone_status: "cloned",
          },
        ]),
      });
    });

    // Mock tasks list
    await page.route("**/api/v1/projects/proj-1/tasks", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "task-1",
            project_id: "proj-1",
            title: "Add API Authentication",
            description: "Implement JWT login and verification endpoints",
            status: "todo",
            complexity: "easy",
            priority: 1,
            labels: ["auth", "backend"],
            spec_status: "none",
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ]),
      });
    });

    // Mock agents list
    await page.route("**/api/v1/projects/proj-1/agents", async (route) => {
      if (route.request().method() === "POST") {
        const body = route.request().postDataJSON();
        const newAgent = {
          id: body.agent_id || "agent-1",
          project_id: "proj-1",
          name: "Hermes Bot",
          role: "backend",
          provider: "google",
          model: "gemini-1.5-pro",
          level: 3,
          status: "idle",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        };
        projectAgents = [newAgent];
        await route.fulfill({
          status: 201,
          contentType: "application/json",
          body: JSON.stringify(newAgent),
        });
      } else {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(projectAgents),
        });
      }
    });

    // Mock skills
    await page.route("**/api/v1/skills", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "skill-1",
            name: "clean-code",
            description: "Clean code guidelines and validation rules",
            schema: {},
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ]),
      });
    });

    // Mock org agents list
    await page.route("**/api/v1/organizations/org-123/agents", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          {
            id: "agent-1",
            org_id: "org-123",
            name: "Hermes Bot",
            role: "backend",
            provider: "google",
            model: "gemini-1.5-pro",
            level: 3,
            status: "idle",
            created_at: new Date().toISOString(),
            updated_at: new Date().toISOString(),
          },
        ]),
      });
    });

    // Mock analytics overview
    await page.route("**/api/v1/analytics/overview**", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          active_projects: 1,
          running_agents: 1,
          open_prs: 0,
          completed_tasks: 0,
        }),
      });
    });
  });

  test("should log in successfully, navigate projects and detail workspace", async ({ page }) => {
    // 1. Go to homepage (Login Form)
    await page.goto("/");
    await expect(page.locator("h1")).toContainText("Auto Code OS");

    // 2. Fill login details
    await page.fill('input[name="email"]', "test@autocodeos.com");
    await page.fill('input[name="password"]', "supersecretpassword");
    await page.click('button[type="submit"]');

    // 3. Land on Projects page and verify mock project renders
    await expect(page.locator("h2")).toContainText("Projects");
    await expect(page.locator("text=Website Refactor")).toBeVisible();

    // 4. Click nav link for Agents
    await page.click("text=Agents");
    await expect(page.locator("select")).toBeVisible();
    // Select project in dropdown
    await page.selectOption("select", "proj-1");
    await expect(page.locator("text=Hermes Bot")).toBeVisible();

    // 5. Click nav link for Skills
    await page.click("text=Skills");
    await expect(page.locator("h2")).toContainText("Skills");
    await expect(page.locator("text=clean-code")).toBeVisible();

    // 6. Click nav link for Rules
    await page.click("text=Rules");
    await expect(page.locator("h2")).toContainText("Rules");
    await expect(page.locator("text=Strict Rules")).toBeVisible();

    // 7. Click nav link for Knowledge
    await page.click("text=Knowledge");
    await expect(page.locator("h2")).toContainText("Knowledge");

    // 8. Click nav link for Organization
    await page.click("text=Organization");
    await expect(page.locator("h2")).toContainText("Organization");
    await expect(page.locator("text=Test Org")).toBeVisible();

    // 9. Go back to projects & navigate to project workspace
    await page.click("text=Projects");
    await page.click("text=Website Refactor");

    // 10. Verify project detail displays repos and tasks
    await expect(page.locator("h1")).toContainText("Website Refactor");
    await expect(page.locator("text=https://github.com/test/repo.git")).toBeVisible();
    await expect(page.locator("text=Add API Authentication")).toBeVisible();
  });
});
