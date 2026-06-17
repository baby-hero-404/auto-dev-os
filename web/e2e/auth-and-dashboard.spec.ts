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
    await expect(page.getByText("github.com/test/repo.git")).toBeVisible();
    await expect(page.getByText("Add API Authentication")).toBeVisible();
  });
});
