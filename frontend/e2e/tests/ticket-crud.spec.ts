import { test, expect } from "@playwright/test";

test.describe("Ticket CRUD", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
  });

  test("ticket list is initially empty", async ({ page }) => {
    await page.goto("/ws/support/tickets");
    await expect(page.getByText("No tickets here yet")).toBeVisible();
  });

  test("create ticket via API and verify in UI", async ({
    page,
    request,
  }) => {
    // Create ticket via API
    const createRes = await request.post(
      "/api/v1/ws/support/tickets",
      {
        data: {
          title: "E2E Test Ticket",
          description: "Created by Playwright E2E test",
        },
      },
    );
    expect(createRes.status()).toBe(201);
    const ticket = await createRes.json();
    expect(ticket.id).toBeTruthy();
    expect(ticket.seqNum).toBeGreaterThanOrEqual(1);
    expect(ticket.title).toBe("E2E Test Ticket");
    const seqLabel = `#${ticket.seqNum}`;

    // Navigate to ticket list
    await page.goto("/ws/support/tickets");
    await expect(page.getByText("E2E Test Ticket")).toBeVisible();
    await expect(
      page.getByRole("main").getByText(seqLabel, { exact: true }),
    ).toBeVisible();

    // Click ticket row to go to detail
    await page.getByText("E2E Test Ticket").click();
    await page.waitForURL(`/ws/support/tickets/${ticket.id}`);

    // Verify detail page
    await expect(page.getByText("E2E Test Ticket")).toBeVisible();
    await expect(
      page.getByText("Created by Playwright E2E test"),
    ).toBeVisible();
    await expect(
      page.getByRole("main").getByText(seqLabel, { exact: true }),
    ).toBeVisible();
  });
});
