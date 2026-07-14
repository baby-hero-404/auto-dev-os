import { expect, test } from "@playwright/test";
import {
  createMockState,
  defaultGitAccount,
  installApiMocks,
  seedSession,
} from "./fixtures/api-mocks";

test.describe("Git accounts credentials management", () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await installApiMocks(
      page,
      createMockState({
        gitAccounts: [defaultGitAccount()],
      }),
    );
  });

  test("manages org Git accounts and exposes them in the repository form", async ({ page }) => {
    await page.goto("/git-accounts");

    await expect(page.getByRole("heading", { name: "Git Accounts", exact: true })).toBeVisible();
    await expect(page.getByText("Default GitHub Dev Account")).toBeVisible();

    const testResponse = page.waitForResponse((response) =>
      response.url().includes("/api/v1/git-accounts/git-acc-1/test") && response.status() === 200,
    );
    await page.getByRole("button", { name: "Test" }).click();
    await testResponse;

    await page.getByRole("button", { name: "Connect Git Account" }).click();
    await expect(page.getByLabel("Display Name")).toBeVisible();

    await page.getByLabel("Provider").selectOption("github");
    await page.getByLabel("Display Name").fill("Enterprise GitHub");
    await page.getByLabel("Base URL").fill("https://github.company.com/api/v3");
    await page.getByLabel("Token").fill("ghp_1234567890");
    await page.getByRole("button", { name: "Connect & Test" }).click();

    await expect(page.getByText("Enterprise GitHub")).toBeVisible();
    await expect(page.getByText("https://github.company.com/api/v3")).toBeVisible();

    await page.getByRole("link", { name: /Projects/i }).click();
    await page.getByRole("link", { name: /Website Refactor/i }).click();

    await page.getByRole("link", { name: "Repositories" }).click();

    await expect(page.getByLabel("Git Account")).toContainText("Enterprise GitHub");

    await page.goto("/git-accounts");

    const enterpriseCard = page.locator("article").filter({ hasText: "Enterprise GitHub" });
    await enterpriseCard.getByTitle("Delete account").click();
    await expect(enterpriseCard.getByText("Delete this account?")).toBeVisible();
    await enterpriseCard.getByRole("button", { name: "Delete", exact: true }).click();

    await expect(page.getByText("Enterprise GitHub")).not.toBeVisible();
    await expect(page.getByText("Default GitHub Dev Account")).toBeVisible();
  });
});
