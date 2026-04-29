import { test, expect } from "@playwright/test";

test.describe("Ticket Kanban (Board) view", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
  });

  test("renders board grouped by status and switches to assignee", async ({
    page,
    request,
  }) => {
    // Seed three tickets across two statuses
    const created: string[] = [];
    for (const title of ["Board A", "Board B", "Board C"]) {
      const res = await request.post("/api/v1/ws/support/tickets", {
        data: { title },
      });
      expect(res.status()).toBe(201);
      const t = await res.json();
      created.push(t.id);
    }
    // Move "Board B" to in-progress so we have two columns populated
    const patch = await request.patch(
      `/api/v1/ws/support/tickets/${created[1]}`,
      { data: { statusId: "in-progress" } },
    );
    expect(patch.ok()).toBeTruthy();

    await page.goto("/ws/support/tickets?view=board");

    const board = page.getByTestId("kanban-scroll");

    // Headers (status names appear in column headers)
    await expect(board.getByText("Open").first()).toBeVisible();
    await expect(board.getByText("In Progress").first()).toBeVisible();

    // The cards we created appear on the board
    await expect(board.getByText("Board A")).toBeVisible();
    await expect(board.getByText("Board B")).toBeVisible();
    await expect(board.getByText("Board C")).toBeVisible();

    // Switch to Group by Assignee
    await page.getByRole("button", { name: /Assignee/ }).first().click();
    // URL should reflect groupBy
    await expect(page).toHaveURL(/groupBy=assignee/);

    // Unassigned lane label should appear (all tickets are unassigned)
    await expect(board.getByText("Unassigned").first()).toBeVisible();
  });

  test("clicking a card navigates to detail", async ({ page, request }) => {
    const res = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Click Me Board" },
    });
    const ticket = await res.json();

    await page.goto("/ws/support/tickets?view=board");
    await page.getByText("Click Me Board").click();
    await page.waitForURL(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Click Me Board")).toBeVisible();
  });

  test("drag-and-drop changes ticket status via PATCH", async ({
    page,
    request,
  }) => {
    const res = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Drag Me Status" },
    });
    const ticket = await res.json();
    expect(ticket.statusId).toBe("open");

    await page.goto("/ws/support/tickets?view=board");

    const card = page.locator(
      `[data-testid="kanban-card"][data-ticket-id="${ticket.id}"]`,
    );
    await expect(card).toBeVisible();

    const columns = page.locator('[data-testid="kanban-column"]');
    // Column index for "in-progress" is 1 (statuses order: open, in-progress, ...)
    const target = columns.nth(1);
    await expect(target).toBeVisible();

    await card.dragTo(target);

    // Wait for PATCH to settle and verify backend state
    await expect
      .poll(async () => {
        const r = await request.get(
          `/api/v1/ws/support/tickets/${ticket.id}`,
        );
        const data = await r.json();
        return data.statusId;
      })
      .toBe("in-progress");
  });
});
