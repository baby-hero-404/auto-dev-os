import { expect, test, type Page } from "@playwright/test";

type GitAccountFixture = {
  id: string;
  org_id: string;
  provider: string;
  display_name: string;
  base_url: string;
  created_at: string;
  updated_at: string;
};

const now = "2026-06-01T00:00:00.000Z";
const orgID = "org-123";
const token = "mock-access-token";
const projectID = "proj-1";

test.describe("Git Accounts credentials management", () => {
  test.beforeEach(async ({ page }) => {
    await seedSession(page);
    await installApiMocks(page);
  });

  test("manages org Git accounts and exposes them in the repository form", async ({ page }) => {
    await page.goto("/settings");

    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
    await page.getByRole("button", { name: /Git Accounts/i }).click();

    await expect(page.getByRole("heading", { name: "Linked Git Accounts" })).toBeVisible();
    await expect(page.getByText("Default GitHub Dev Account")).toBeVisible();

    await page.getByRole("button", { name: "Test" }).click();
    await expect(page.getByRole("button", { name: "Success" })).toBeVisible();

    await page.getByRole("button", { name: "Add Account" }).click();
    await expect(page.getByRole("heading", { name: "Add Git Account" })).toBeVisible();

    await page.getByLabel("Display Name").fill("Enterprise GitHub");
    await page.getByLabel("Provider").selectOption("github");
    await page.getByLabel("GitHub API Base URL (Optional)").fill("https://github.company.com/api/v3");
    await page.getByLabel("Personal Access Token").fill("ghp_1234567890");
    await page.getByRole("button", { name: "Save" }).click();

    await expect(page.getByText("Enterprise GitHub")).toBeVisible();
    await expect(page.getByText("https://github.company.com/api/v3")).toBeVisible();

    await page.getByRole("link", { name: /Projects/i }).click();
    await page.getByRole("link", { name: /Website Refactor/i }).click();

    await expect(page.getByText("Git Account Credential")).toBeVisible();
    await expect(page.getByRole("combobox").filter({ hasText: "None / Manual Override Token" })).toContainText(
      "Enterprise GitHub (GitHub Enterprise)",
    );

    await page.goto("/settings");
    await page.getByRole("button", { name: /Git Accounts/i }).click();

    const enterpriseCard = page.locator("article").filter({ hasText: "Enterprise GitHub" });
    await enterpriseCard.getByTitle("Delete account").click();
    await expect(enterpriseCard.getByText("Confirm deletion?")).toBeVisible();
    await enterpriseCard.getByRole("button", { name: "Yes, Delete" }).click();

    await expect(page.getByText("Enterprise GitHub")).not.toBeVisible();
    await expect(page.getByText("Default GitHub Dev Account")).toBeVisible();
  });
});

async function seedSession(page: Page) {
  await page.addInitScript(
    ({ sessionToken, sessionOrgID }) => {
      window.localStorage.setItem(
        "autocodeos.session",
        JSON.stringify({
          token: sessionToken,
          user: {
            id: "u-123",
            email: "test@autocodeos.com",
            org_id: sessionOrgID,
            role: "admin",
          },
        }),
      );
    },
    { sessionToken: token, sessionOrgID: orgID },
  );
}

async function installApiMocks(page: Page) {
  let gitAccounts: GitAccountFixture[] = [
    {
      id: "git-acc-1",
      org_id: orgID,
      provider: "github",
      display_name: "Default GitHub Dev Account",
      base_url: "",
      created_at: now,
      updated_at: now,
    },
  ];

  await page.route("**/api/v1/health", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ status: "ok", version: "0.2.0" }),
    });
  });

  await page.route(`**/api/v1/organizations/${orgID}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ id: orgID, name: "Test Org", created_at: now, updated_at: now }),
    });
  });

  await page.route(`**/api/v1/organizations/${orgID}/projects`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([
        {
          id: projectID,
          org_id: orgID,
          name: "Website Refactor",
          description: "Refactoring legacy website code",
          created_at: now,
          updated_at: now,
        },
      ]),
    });
  });

  await page.route(`**/api/v1/projects/${projectID}`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        id: projectID,
        org_id: orgID,
        name: "Website Refactor",
        description: "Refactoring legacy website code",
        created_at: now,
        updated_at: now,
      }),
    });
  });

  await page.route(`**/api/v1/projects/${projectID}/repositories`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route(`**/api/v1/projects/${projectID}/tasks`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route(`**/api/v1/projects/${projectID}/agents`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route(`**/api/v1/projects/${projectID}/rules`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route(`**/api/v1/organizations/${orgID}/agents`, async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route("**/api/v1/skills", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify([]),
    });
  });

  await page.route("**/api/v1/analytics/overview**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        total_projects: 1,
        active_tasks: 0,
        running_agents: 0,
        open_prs: 0,
      }),
    });
  });

  await page.route(`**/api/v1/organizations/${orgID}/git-accounts`, async (route) => {
    if (route.request().method() === "POST") {
      const payload = route.request().postDataJSON() as {
        provider: string;
        display_name: string;
        base_url?: string;
      };
      const created: GitAccountFixture = {
        id: "git-acc-2",
        org_id: orgID,
        provider: payload.provider,
        display_name: payload.display_name,
        base_url: payload.base_url || "",
        created_at: now,
        updated_at: now,
      };
      gitAccounts = [...gitAccounts, created];
      await route.fulfill({
        status: 201,
        contentType: "application/json",
        body: JSON.stringify(created),
      });
      return;
    }

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(gitAccounts),
    });
  });

  await page.route("**/api/v1/git-accounts/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    const segments = url.pathname.split("/");
    const accountID = segments[segments.indexOf("git-accounts") + 1];

    if (request.method() === "POST" && url.pathname.endsWith("/test")) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "success" }),
      });
      return;
    }

    if (request.method() === "DELETE") {
      gitAccounts = gitAccounts.filter((account) => account.id !== accountID);
      await route.fulfill({ status: 204 });
      return;
    }

    await route.fulfill({
      status: 404,
      contentType: "application/json",
      body: JSON.stringify({ error: "Unhandled git account route" }),
    });
  });
}
