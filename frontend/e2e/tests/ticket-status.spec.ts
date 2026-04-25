import { test, expect } from "@playwright/test";

test.describe("Ticket Status Change", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
  });

  test("change ticket status from detail page", async ({
    page,
    request,
  }) => {
    // Create a ticket
    const createRes = await request.post(
      "/api/v1/ws/support/tickets",
      {
        data: { title: "Status Change Test" },
      },
    );
    expect(createRes.status()).toBe(201);
    const ticket = await createRes.json();

    // Default status should be "open"
    expect(ticket.statusId).toBe("open");

    // Go to detail page
    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Status Change Test")).toBeVisible();

    // Status dropdown trigger should display the current status "Open"
    const statusTrigger = page.getByRole("button", { name: /Open/ });
    await expect(statusTrigger).toBeVisible();

    // Open the status dropdown
    await statusTrigger.click();

    // Click "In Progress" option in the dropdown
    await page.getByRole("button", { name: /In Progress/ }).click();

    // The dropdown trigger should now display "In Progress"
    await expect(
      page.getByRole("button", { name: /In Progress/ }),
    ).toBeVisible();

    // Verify via API
    const getRes = await request.get(
      `/api/v1/ws/support/tickets/${ticket.id}`,
    );
    const updated = await getRes.json();
    expect(updated.statusId).toBe("in-progress");
  });
});
