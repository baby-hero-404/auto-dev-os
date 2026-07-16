import { expect, test } from "@playwright/test";
import {
  createMockState,
  installApiMocks,
  seedSession,
  projectID,
} from "./fixtures/api-mocks";

test.describe("Task Detail UI Enhancements", () => {
  test.beforeEach(async ({ page }) => {
    page.on("console", (msg) => {
      console.log(`[BROWSER]: ${msg.text()}`);
    });
    await seedSession(page);
    await installApiMocks(
      page,
      createMockState()
    );
  });

  test("renders all new dashboard elements and handles workflow controls", async ({ page }) => {
    // Navigate to the task detail page for task-1
    await page.goto(`/projects/${projectID}/tasks/task-1`);

    // Clean up task-specific localStorage keys from previous runs
    await page.evaluate(() => {
      for (let i = window.localStorage.length - 1; i >= 0; i--) {
        const key = window.localStorage.key(i);
        if (key && key.startsWith("task-")) {
          window.localStorage.removeItem(key);
        }
      }
    });

    // Verify task header and title
    await expect(page.getByRole("heading", { name: "Add API Authentication" })).toBeVisible();

    // Verify DashboardSummary elements
    const summaryCard = page.locator("div.bg-card").filter({ hasText: /Status/i }).first();
    await expect(summaryCard).toBeVisible();
    await expect(summaryCard.getByText("Todo")).toBeVisible();
    await expect(summaryCard.getByText("Write database models")).toBeVisible();
    await expect(summaryCard.getByText("0%")).toBeVisible();
    await expect(summaryCard.getByText("0 / 2")).toBeVisible();

    // Verify ActiveWorkflowBanner elements
    const activeHeading = page.getByRole("heading", { name: "AI is currently working on" });
    await expect(activeHeading).toBeVisible();
    await expect(activeHeading).toContainText("Write database models");

    // Verify Workflow Sidebar Accordions: Checkpoints should start collapsed
    const checkpointsCard = page.locator("aside div.rounded-xl.border.border-stroke.bg-card.p-5").filter({ hasText: "Checkpoints" });
    const checkpointsButton = checkpointsCard.getByRole("button", { name: /Checkpoints/ });
    await expect(checkpointsButton).toBeVisible();
    await expect(checkpointsCard.getByText(/2026/).first()).not.toBeVisible();

    // Toggle checkpoints expansion
    await checkpointsButton.click();
    await expect(checkpointsCard.getByText(/2026/).first()).toBeVisible();

    // Verify SpecPanel Collapsibility
    // Risks Assessment should be expanded by default (since risk_domains has 'security')
    await expect(page.getByText("Token leak")).toBeVisible();
    
    // Execution Boundaries should be collapsed by default
    await expect(page.locator("button:has-text('Execution Boundaries') + div")).toHaveCSS("max-height", "0px");
    // Expand boundaries
    await page.getByRole("button", { name: /Execution Boundaries/i }).click();
    await expect(page.locator("button:has-text('Execution Boundaries') + div")).toHaveCSS("max-height", "400px");

    // Header Quick Actions: Click "Pause" and wait for POST request
    const pauseResponse = page.waitForResponse((response) =>
      response.url().includes("/api/v1/tasks/task-1/pause") && response.status() === 200
    );
    await page.getByRole("button", { name: "Pause" }).click();
    await pauseResponse;
  });

  test("verifies layout ordering, collapse persistence, and scroll-to-log click interactions", async ({ page }) => {
    // Navigate to the task detail page
    await page.goto(`/projects/${projectID}/tasks/task-1`);

    // Clean up task-specific localStorage keys from previous runs
    await page.evaluate(() => {
      for (let i = window.localStorage.length - 1; i >= 0; i--) {
        const key = window.localStorage.key(i);
        if (key && key.startsWith("task-")) {
          window.localStorage.removeItem(key);
        }
      }
    });

    // 1. Verify layout ordering (HUD cards/checklist at the top, Logs on left, Spec/PR panels on right)
    // Left column: LogConsole
    const logConsole = page.locator("#log-console");
    await expect(logConsole).toBeVisible();

    // Right column: SpecPanel
    const specPanel = page.locator("button:has-text('Execution Boundaries')");
    await expect(specPanel).toBeVisible();

    // 2. Verify Collapsibility & Persistence
    // "Scope" description should start collapsed
    const scopeParagraph = page.locator("div.relative:has-text('Scope') div.overflow-hidden");
    await expect(scopeParagraph).toHaveCSS("max-height", "42px"); // 3em = 42px on 14px base font

    // Click "Show Description"
    await page.getByRole("button", { name: "Show Description" }).click();
    await expect(scopeParagraph).toHaveCSS("max-height", "1000px");

    // Click "Hide Description"
    await page.getByRole("button", { name: "Hide Description" }).click();
    await expect(scopeParagraph).toHaveCSS("max-height", "42px");

    // Persistence: Click "Show Description" to expand it
    await page.getByRole("button", { name: "Show Description" }).click();
    await expect(scopeParagraph).toHaveCSS("max-height", "1000px");
    
    // Risks starts expanded because of the security risk_domain. Wait for it to load.
    await expect(page.getByText("Token leak")).toBeVisible();
    // Let's collapse it.
    await page.getByRole("button", { name: /Risks Assessment/i }).click();
    await expect(page.locator("button:has-text('Risks Assessment') + div")).toHaveCSS("max-height", "0px");
    
    // Boundaries starts collapsed. Let's expand it.
    await page.getByRole("button", { name: /Execution Boundaries/i }).click();
    await expect(page.locator("button:has-text('Execution Boundaries') + div")).toHaveCSS("max-height", "400px");

    // Reload the page and check if state persisted in localStorage
    await page.reload();
    
    // Description should still be expanded (maxHeight 1000px)
    await expect(page.locator("div.relative:has-text('Scope') div.overflow-hidden")).toHaveCSS("max-height", "1000px");
    
    // Risks should still be collapsed (max-height 0px)
    await expect(page.locator("button:has-text('Risks Assessment') + div")).toHaveCSS("max-height", "0px");

    // Boundaries should still be expanded (max-height 400px)
    await expect(page.locator("button:has-text('Execution Boundaries') + div")).toHaveCSS("max-height", "400px");

    // 3. Scroll Interactions
    // Verify checklist item clicking works
    const checklistItem = page.locator("section:has-text('Implementation Checklist')")
      .locator("div.cursor-pointer")
      .filter({ hasText: "Write database models" })
      .first();
    await expect(checklistItem).toBeVisible();
    await checklistItem.click();

    // Verify timeline milestone node clicking works
    const timelineMilestone = page.locator("div.group.relative")
      .filter({ hasText: /context load/i })
      .locator("div.cursor-pointer")
      .first();
    await expect(timelineMilestone).toBeVisible();
    await timelineMilestone.click();
  });

  test("verifies sub-timeline, log milestones toggle, and live action indicator", async ({ page }) => {
    // Navigate to the task detail page
    await page.goto(`/projects/${projectID}/tasks/task-1`);

    // Clean up task-specific localStorage keys from previous runs
    await page.evaluate(() => {
      for (let i = window.localStorage.length - 1; i >= 0; i--) {
        const key = window.localStorage.key(i);
        if (key && key.startsWith("task-")) {
          window.localStorage.removeItem(key);
        }
      }
    });

    // 1. Verify Sub-timeline component is present under Phase 1 / Group node
    // The code node on the timeline should show "Sub-Tasks (2)"
    const subTasksToggle = page.locator("button:has-text('Sub-Tasks')").first();
    await expect(subTasksToggle).toBeVisible();
    await expect(subTasksToggle).toContainText("Sub-Tasks (2)");

    // It starts expanded because the workflow is not success/failed/running (status todo)
    // Let's verify the two sub-items are visible
    await expect(page.getByText("Write database models").first()).toBeVisible();
    await expect(page.getByText("Build login page").first()).toBeVisible();

    // Toggle collapse
    await subTasksToggle.click();
    // After collapse, the sub-timeline container should be hidden (max-height 0px)
    await expect(page.locator("button:has-text('Sub-Tasks') + div").first()).toHaveCSS("max-height", "0px");

    // 2. Verify LogConsole Milestones/All Logs toggle
    const milestonesTab = page.locator("button:has-text('Milestones')").first();
    const allLogsTab = page.locator("button:has-text('All Logs')").first();
    await expect(milestonesTab).toBeVisible();
    await expect(allLogsTab).toBeVisible();

    // Click "Milestones" tab
    await milestonesTab.click();
    
    // In milestones mode, we should see the parsed checkpoint log
    await expect(page.getByText("checkpoint: load success").first()).toBeVisible();

    // 3. Verify Live Action Indicator is not visible when workflow is not running
    const actionIndicator = page.locator("span:has-text('Editing')");
    await expect(actionIndicator).not.toBeVisible();
  });
});
