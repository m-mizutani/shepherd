import { test, expect, type Page } from "@playwright/test";

const FAKE_USERS = [
  { id: "U_ALICE", name: "Alice", email: "alice@example.com" },
  { id: "U_BOB", name: "Bob", email: "bob@example.com" },
];

async function stubSlackUsers(page: Page) {
  await page.route("**/api/v1/ws/*/slack/users", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ users: FAKE_USERS }),
    });
  });
  await page.route("**/api/v1/ws/*/slack/users/*", async (route) => {
    const url = new URL(route.request().url());
    const id = url.pathname.split("/").pop()!;
    const u = FAKE_USERS.find((x) => x.id === id);
    if (u) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ id: u.id, name: u.name, email: u.email }),
      });
    } else {
      await route.fulfill({ status: 404, body: "" });
    }
  });
}

test.describe("Ticket Assignee inline edit (multi-user)", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
    await stubSlackUsers(page);
  });

  test("a single assignee can be added without entering Edit mode", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Inline Assignee Test" },
    });
    expect(createRes.status()).toBe(201);
    const ticket = await createRes.json();

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Inline Assignee Test")).toBeVisible();

    // Edit button must still be visible (we are NOT in Edit mode).
    const editBtn = page.getByRole("button", { name: /^Edit$/ });
    await expect(editBtn).toBeVisible();

    // Open assignee picker by focusing its empty input.
    const picker = page.getByPlaceholder("Unassigned");
    await expect(picker).toBeVisible();
    await picker.click();

    // Capture the PATCH request issued by selecting a user.
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await page.getByRole("option", { name: "Alice" }).click();
    const req = await patchReq;

    // Body must contain ONLY assigneeIds — nothing else.
    const body = JSON.parse(req.postData() ?? "{}");
    expect(body).toEqual({ assigneeIds: ["U_ALICE"] });

    // We must not have entered Edit mode.
    await expect(
      page.getByRole("button", { name: /Save changes/ }),
    ).toHaveCount(0);

    // Server-side: only assignees changed.
    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeIds).toEqual(["U_ALICE"]);
    expect(after.title).toBe("Inline Assignee Test");
    expect(after.statusId).toBe(ticket.statusId);
    expect(after.description ?? "").toBe(ticket.description ?? "");
  });

  test("a second assignee can be added on top of an existing one", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Two Assignees Test" },
    });
    const ticket = await createRes.json();
    await request.patch(`/api/v1/ws/support/tickets/${ticket.id}`, {
      data: { assigneeIds: ["U_ALICE"] },
    });

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Two Assignees Test")).toBeVisible();

    // Click the picker container — clicking on the chip-area div opens the
    // dropdown without removing the existing chip.
    await page.getByText("Alice", { exact: true }).first().click();

    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await page.getByRole("option", { name: "Bob" }).click();
    const req = await patchReq;
    expect(JSON.parse(req.postData() ?? "{}")).toEqual({
      assigneeIds: ["U_ALICE", "U_BOB"],
    });

    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeIds).toEqual(["U_ALICE", "U_BOB"]);
  });

  test("removing one of two assignees keeps the rest", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Remove One Test" },
    });
    const ticket = await createRes.json();
    await request.patch(`/api/v1/ws/support/tickets/${ticket.id}`, {
      data: { assigneeIds: ["U_ALICE", "U_BOB"] },
    });

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Remove One Test")).toBeVisible();

    // Click the Remove button next to Alice's chip. The picker renders one
    // Remove button per chip; we target the one whose neighbour is Alice.
    const aliceChip = page
      .locator("span")
      .filter({ hasText: /^Alice$/ })
      .first()
      .locator("..");
    const removeAlice = aliceChip.getByRole("button", { name: "Remove" });
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await removeAlice.click();
    const req = await patchReq;
    expect(JSON.parse(req.postData() ?? "{}")).toEqual({
      assigneeIds: ["U_BOB"],
    });

    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeIds).toEqual(["U_BOB"]);
  });

  test("removing the last assignee clears the list to empty", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Unassign Test" },
    });
    const ticket = await createRes.json();
    await request.patch(`/api/v1/ws/support/tickets/${ticket.id}`, {
      data: { assigneeIds: ["U_ALICE"] },
    });

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Unassign Test")).toBeVisible();

    const aliceChip = page
      .locator("span")
      .filter({ hasText: /^Alice$/ })
      .first()
      .locator("..");
    const removeAlice = aliceChip.getByRole("button", { name: "Remove" });
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await removeAlice.click();
    const req = await patchReq;
    expect(JSON.parse(req.postData() ?? "{}")).toEqual({ assigneeIds: [] });

    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeIds ?? []).toEqual([]);
    expect(after.title).toBe("Unassign Test");
  });

  test("inline assignee change while in Edit mode does not close Edit mode or lose unsaved input", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Mid-Edit Test", description: "original-desc" },
    });
    const ticket = await createRes.json();

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Mid-Edit Test")).toBeVisible();

    // Enter Edit mode and stage unsaved changes in title + description.
    await page.getByRole("button", { name: /^Edit$/ }).click();
    const titleInput = page.locator("input").filter({ hasText: "" }).first();
    await titleInput.fill("Mid-Edit Test (unsaved)");
    await page.locator("textarea").first().fill("unsaved-desc");

    // Inline-edit assignee while still in Edit mode.
    await page.getByPlaceholder("Unassigned").click();
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await page.getByRole("option", { name: "Alice" }).click();
    expect(JSON.parse((await patchReq).postData() ?? "{}")).toEqual({
      assigneeIds: ["U_ALICE"],
    });

    // Save changes button must still be visible — Edit mode stayed open.
    await expect(
      page.getByRole("button", { name: /Save changes/ }),
    ).toBeVisible();
    // Unsaved title/description must still be present in the inputs.
    await expect(titleInput).toHaveValue("Mid-Edit Test (unsaved)");
    await expect(page.locator("textarea").first()).toHaveValue("unsaved-desc");

    // Server-side: title/description still match the original (not yet saved).
    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeIds).toEqual(["U_ALICE"]);
    expect(after.title).toBe("Mid-Edit Test");
    expect(after.description).toBe("original-desc");
  });
});
