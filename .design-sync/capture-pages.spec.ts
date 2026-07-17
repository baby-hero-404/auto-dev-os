import { mkdirSync } from "node:fs";
import { test } from "@playwright/test";
import {
  createMockState,
  defaultAgent,
  installApiMocks,
  projectID,
  seedSession,
} from "../web/e2e/fixtures/api-mocks";

// Maintenance script for the design-sync "page references" (see
// .design-sync/NOTES.md → "Page references"). Not part of the real e2e
// suite — copy into web/e2e/ to run, then delete. See NOTES.md for the
// exact command.
const OUT_DIR = "../.design-sync/pages";
mkdirSync(OUT_DIR, { recursive: true });

const routes: { path: string; name: string }[] = [
  { path: "/", name: "Home" },
  { path: "/agents", name: "Agents" },
  { path: "/ai-providers", name: "AI-Providers" },
  { path: "/analytics", name: "Analytics" },
  { path: "/audit", name: "Audit" },
  { path: "/gateway", name: "Gateway" },
  { path: "/git-accounts", name: "Git-Accounts" },
  { path: "/knowledge", name: "Knowledge" },
  { path: "/knowledge/suggestions", name: "Knowledge-Suggestions" },
  { path: "/organization", name: "Organization" },
  { path: "/rules", name: "Rules" },
  { path: "/settings", name: "Settings" },
  { path: "/skills", name: "Skills" },
  { path: `/projects/${projectID}`, name: "Project-Workspace" },
  { path: `/projects/${projectID}/tasks/task-1`, name: "Task-Detail" },
];

test.describe("page reference capture", () => {
  test.beforeEach(async ({ page }) => {
    // Registered first so the specific mocks installApiMocks adds afterwards
    // take precedence (Playwright: last-registered route wins on overlap).
    await page.route("**/api/v1/**", async (route) => {
      const method = route.request().method();
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: method === "GET" ? "[]" : "{}",
      });
    });
    await seedSession(page);
    await installApiMocks(
      page,
      createMockState({
        projectAgents: [
          defaultAgent({ org_id: undefined, project_id: projectID, autonomy_level: "autonomous" }),
        ],
      }),
    );
  });

  for (const { path, name } of routes) {
    test(`capture ${name}`, async ({ page }) => {
      await page.goto(path);
      await page.waitForLoadState("networkidle");
      await page.waitForTimeout(500);
      await page.screenshot({ path: `${OUT_DIR}/${name}.png`, fullPage: true });
    });
  }
});
