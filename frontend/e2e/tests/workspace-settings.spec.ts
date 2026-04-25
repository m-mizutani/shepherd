import { test, expect } from "@playwright/test";

test.describe("Workspace Settings (Read-Only)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
  });

  test("displays status definitions", async ({ page }) => {
    await page.goto("/ws/support/settings");

    await expect(page.getByRole("heading", { name: "Statuses" })).toBeVisible();
    // Statuses from examples/config.toml
    const tbody = page.locator("table").first().locator("tbody");
    await expect(tbody.getByText("Open", { exact: true })).toBeVisible();
    await expect(tbody.getByText("In Progress", { exact: true })).toBeVisible();
    await expect(tbody.getByText("Waiting", { exact: true })).toBeVisible();
    await expect(tbody.getByText("Resolved", { exact: true })).toBeVisible();
    await expect(tbody.getByText("Closed", { exact: true })).toBeVisible();
  });

  test("displays field definitions", async ({ page }) => {
    await page.goto("/ws/support/settings");
    await page.getByRole("button", { name: "Fields", exact: true }).click();

    await expect(page.getByRole("heading", { name: "Custom Fields" })).toBeVisible();
    // Fields from examples/config.toml: priority, category, due-date, reference-url
    const fieldsSection = page.locator("section").filter({ hasText: "Custom Fields" });
    const fieldsTable = fieldsSection.locator("table");
    await expect(fieldsTable.getByRole("cell", { name: "Priority", exact: true })).toBeVisible();
    await expect(fieldsTable.getByRole("cell", { name: "Category", exact: true })).toBeVisible();
    await expect(fieldsTable.getByRole("cell", { name: "Due Date", exact: true })).toBeVisible();
    await expect(fieldsTable.getByRole("cell", { name: "Reference URL", exact: true })).toBeVisible();
  });

  test("displays ticket config", async ({ page }) => {
    await page.goto("/ws/support/settings");
    // Activate the Ticket Config section in the side nav
    await page.getByRole("button", { name: "Ticket Config" }).click();

    await expect(page.getByRole("heading", { name: "Ticket Config" })).toBeVisible();
    const section = page.locator("section").filter({ hasText: "Ticket Config" });
    await expect(section.getByText("Default Status")).toBeVisible();
    await expect(section.getByText("Open", { exact: true })).toBeVisible();
    await expect(section.getByText("Closed Statuses")).toBeVisible();
    await expect(section.getByText("Resolved", { exact: true })).toBeVisible();
    await expect(section.getByText("Closed", { exact: true })).toBeVisible();
  });

  test("displays labels", async ({ page }) => {
    await page.goto("/ws/support/settings");
    await page.getByRole("button", { name: "Labels" }).click();

    await expect(page.getByRole("heading", { name: "Labels" })).toBeVisible();
  });
});
