import { expect, test } from "@playwright/test";
import {
  attachBrowserDiagnostics,
  createMockState,
  defaultAgent,
  installApiMocks,
} from "./fixtures/api-mocks";

test.describe("Auto Code OS E2E flows", () => {
  test.beforeEach(async ({ page }) => {
    attachBrowserDiagnostics(page, "auth");
    await installApiMocks(
      page,
      createMockState({
        projectAgents: [defaultAgent({ org_id: undefined, project_id: "proj-1", autonomy_level: "autonomous" })],
      }),
    );
  });

  test("logs in, navigates primary sections, and opens project workspace", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByRole("heading", { name: /Auto Code OS/i })).toBeVisible();

    await page.getByLabel(/email/i).fill("test@autocodeos.com");
    await page.getByLabel(/password/i).fill("supersecretpassword");
    await page.getByRole("button", { name: "Continue" }).click();

    await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();
    await expect(page.getByRole("link", { name: /Website Refactor/i })).toBeVisible();

    await page.getByRole("link", { name: "Agents", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Agents" })).toBeVisible();
    await expect(page.getByText("Hermes Bot")).toBeVisible();
    await expect(page.getByText("Website Refactor")).toBeVisible();

    await page.getByRole("link", { name: "Skills", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Skills" })).toBeVisible();
    await expect(page.getByText("clean-code")).toBeVisible();

    await page.getByRole("link", { name: "Rules", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Global Rules", level: 2 })).toBeVisible();
    await expect(page.getByText("No console.logs")).toBeVisible();

    await page.getByRole("link", { name: "Knowledge", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Knowledge" })).toBeVisible();

    await page.getByRole("link", { name: "Organization", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Organization", exact: true })).toBeVisible();
    await expect(page.getByText("Test Org")).toBeVisible();

    await page.getByRole("link", { name: "Projects", exact: true }).click();
    await page.getByRole("link", { name: /Website Refactor/i }).click();

    await expect(page.getByRole("heading", { name: "Website Refactor" })).toBeVisible();
    await page.getByRole("button", { name: "Repositories" }).click();
    await expect(page.getByText("github.com/test/repo.git")).toBeVisible();
  });

  test("shows only the three agent model levels in the hire wizard", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel(/email/i).fill("test@autocodeos.com");
    await page.getByLabel(/password/i).fill("supersecretpassword");
    await page.getByRole("button", { name: "Continue" }).click();

    await page.getByRole("link", { name: "Agents", exact: true }).click();
    await page.getByRole("button", { name: "Hire Agent" }).click();
    await page.getByLabel(/name/i).fill("Quality Lead");
    await page.getByLabel(/goal/i).fill("Validate the available model levels.");
    await page.getByRole("button", { name: "Next", exact: true }).click();

    const modelLevel = page.getByRole("combobox", { name: "Model Intelligence Level" });
    await expect(modelLevel).toBeVisible();
    await expect(modelLevel.locator("option")).toHaveCount(3);
    await expect(modelLevel.locator("option")).toHaveText(["fast", "balanced", "powerful"]);
  });

  test("displays key level analytics report on the dashboard", async ({ page }) => {
    await page.goto("/");
    await page.getByLabel(/email/i).fill("test@autocodeos.com");
    await page.getByLabel(/password/i).fill("supersecretpassword");
    await page.getByRole("button", { name: "Continue" }).click();

    await page.getByRole("link", { name: "Analytics", exact: true }).click();
    await expect(page.getByRole("heading", { name: "Analytics" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "Virtual Key Usage & Spend" })).toBeVisible();
    await expect(page.getByRole("heading", { name: "API Request Success Rate" })).toBeVisible();
    await expect(page.getByText("OpenAI-Prod-Key1")).toBeVisible();
  });
});
