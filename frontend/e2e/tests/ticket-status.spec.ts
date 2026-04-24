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

    // The "Open" status button should be currently active (styled differently)
    const openButton = page.getByRole("button", { name: "Open" });
    await expect(openButton).toBeDisabled();

    // Click "In Progress" status button
    const inProgressButton = page.getByRole("button", {
      name: "In Progress",
    });
    await inProgressButton.click();

    // Wait for the mutation to complete - "In Progress" should now be disabled (active)
    await expect(inProgressButton).toBeDisabled();
    // "Open" should now be enabled (clickable)
    await expect(openButton).toBeEnabled();

    // Verify via API
    const getRes = await request.get(
      `/api/v1/ws/support/tickets/${ticket.id}`,
    );
    const updated = await getRes.json();
    expect(updated.statusId).toBe("in-progress");
  });
});
