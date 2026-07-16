import { expect, test, type Page } from "@playwright/test";
import {
  createMockState,
  installApiMocks,
  seedSession,
  projectID,
} from "./fixtures/api-mocks";

const taskUrl = `/projects/${projectID}/tasks/task-1`;

async function clearTaskLocalStorage(page: Page) {
  await page.evaluate(() => {
    for (let i = window.localStorage.length - 1; i >= 0; i--) {
      const key = window.localStorage.key(i);
      if (key && key.startsWith("task-")) {
        window.localStorage.removeItem(key);
      }
    }
  });
}

test.describe("Task Detail — workflow-oriented redesign", () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await installApiMocks(page, createMockState());
  });

  test("workflow-first composition: checklist above phases, spec/log/description collapsed by default", async ({ page }) => {
    await page.goto(taskUrl);
    await clearTaskLocalStorage(page);

    // Header
    await expect(page.getByRole("heading", { name: "Add API Authentication" })).toBeVisible();

    // REQ-007: project description collapsed by default
    await expect(page.getByRole("button", { name: /Show project description/i })).toBeVisible();
    await expect(page.getByText("Implement JWT login and verification endpoints")).toHaveCount(0);
    await page.getByRole("button", { name: /Show project description/i }).click();
    await expect(page.getByText("Implement JWT login and verification endpoints")).toBeVisible();

    // REQ-003: implementation checklist is the primary progress surface, shows the running unit
    const checklistHeading = page.getByRole("heading", { name: "Implementation Checklist" });
    await expect(checklistHeading).toBeVisible();
    await expect(page.getByText("Write database models").first()).toBeVisible();

    // REQ-002/REQ-004: checklist renders ABOVE the (demoted) Workflow Phases timeline
    const phasesHeading = page.getByRole("heading", { name: "Workflow Phases" });
    await expect(phasesHeading).toBeVisible();
    const checklistBox = await checklistHeading.boundingBox();
    const phasesBox = await phasesHeading.boundingBox();
    expect(checklistBox!.y).toBeLessThan(phasesBox!.y);

    // REQ-005: SpecPanel collapsed by default — title + "View details", body hidden
    await expect(page.getByRole("button", { name: /Proposed Task Specification/i })).toBeVisible();
    await expect(page.getByText("View details")).toBeVisible();
    await expect(page.getByRole("button", { name: /Execution Boundaries/i })).toHaveCount(0);
    await page.getByRole("button", { name: /Proposed Task Specification/i }).click();
    await expect(page.getByRole("button", { name: /Execution Boundaries/i })).toBeVisible();

    // REQ-006: LogConsole collapsed by default — "View full log", body hidden
    await expect(page.getByText("View full log")).toBeVisible();
    await expect(page.getByRole("button", { name: "All Logs" })).toHaveCount(0);
    await page.getByText("View full log").click();
    // Expanding reveals the full log body (Milestones/All Logs toggle + parsed milestone).
    await expect(page.getByRole("button", { name: "All Logs" })).toBeVisible();
    await expect(page.getByText("checkpoint: load success").first()).toBeVisible();

    // REQ-008/009/010: removed surfaces must not appear
    await expect(page.getByRole("heading", { name: /AI is currently working on/i })).toHaveCount(0);
    await expect(page.getByRole("button", { name: /^Checkpoints/i })).toHaveCount(0);
    await expect(page.getByText("Task Controls")).toHaveCount(0);
  });

  test("sticky review bar shows only when a decision is pending and fires Approve Spec", async ({ page }) => {
    // Override the workflow route with a self-contained body in spec-review state.
    await page.route("**/api/v1/tasks/task-1/workflow", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          task: {
            id: "task-1",
            project_id: projectID,
            title: "Add API Authentication",
            description: "Implement JWT login and verification endpoints",
            status: "spec_review",
            spec_status: "pending_review",
            priority: 1,
            analysis: {
              scope: "Implement JWT auth endpoints",
              execution_units: [
                { id: "code_backend_0", objective: "Write database models", tasks: [], execution_profile: { type: "backend" }, constraints: { max_files: 2, estimated_tokens: 100, max_risk: "low" } },
              ],
            },
          },
          job: { id: "job-1", status: "paused", step: "analyze", progress: 30, created_at: new Date().toISOString(), updated_at: new Date().toISOString() },
          checkpoints: [],
        }),
      });
    });

    await page.goto(taskUrl);
    await clearTaskLocalStorage(page);

    // REQ-001: sticky bar present with both spec-review CTAs
    await expect(page.getByText("Waiting for your review")).toBeVisible();
    const approve = page.getByRole("button", { name: "Approve Spec" });
    await expect(approve).toBeVisible();
    await expect(page.getByRole("button", { name: "Request Changes" })).toBeVisible();

    // Clicking Approve Spec fires the analysis/approve endpoint
    const approveResponse = page.waitForResponse((r) =>
      r.url().includes("/api/v1/tasks/task-1/analysis/approve") && r.request().method() === "POST"
    );
    await approve.click();
    await approveResponse;
  });

  test("no sticky review bar when no decision is pending (default running state)", async ({ page }) => {
    await page.goto(taskUrl);
    await clearTaskLocalStorage(page);
    // Default fixture: spec_status "review" (not pending_review) + job running → no bar.
    await expect(page.getByText("Waiting for your review")).toHaveCount(0);
  });
});
