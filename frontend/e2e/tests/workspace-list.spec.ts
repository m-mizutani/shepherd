import { test, expect } from "@playwright/test";

test.describe("Workspace List", () => {
  test.beforeEach(async ({ page }) => {
    // Login via NoAuthn
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
  });

  test("displays workspace cards", async ({ page }) => {
    await expect(page.getByText("Workspaces")).toBeVisible();
    // examples/config.toml defines "support" workspace with name "Support Team"
    await expect(page.getByRole("heading", { name: "Support Team" })).toBeVisible();
    await expect(page.getByText("support", { exact: true })).toBeVisible();
  });

  test("clicking workspace navigates to ticket list", async ({ page }) => {
    await page.getByRole("heading", { name: "Support Team" }).click();
    await page.waitForURL("/ws/support/tickets");
    await expect(page.getByText("Tickets")).toBeVisible();
  });
});
