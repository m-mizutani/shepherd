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

test.describe("Ticket Assignee inline edit", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
    await stubSlackUsers(page);
  });

  test("assignee can be changed without entering Edit mode", async ({
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

    // Open assignee picker (no Edit mode).
    const picker = page.getByRole("button", { name: /Unassigned/i });
    await expect(picker.first()).toBeVisible();
    await picker.first().click();

    // Capture the PATCH request issued by selecting a user.
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await page.getByRole("option", { name: "Alice" }).click();
    const req = await patchReq;

    // Body must contain ONLY assigneeId — nothing else.
    const body = JSON.parse(req.postData() ?? "{}");
    expect(body).toEqual({ assigneeId: "U_ALICE" });

    // We must not have entered Edit mode.
    await expect(
      page.getByRole("button", { name: /Save changes/ }),
    ).toHaveCount(0);

    // Server-side: only assignee changed.
    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeId).toBe("U_ALICE");
    expect(after.title).toBe("Inline Assignee Test");
    expect(after.statusId).toBe(ticket.statusId);
    expect(after.description ?? "").toBe(ticket.description ?? "");
  });

  test("inline assignee change does not touch other fields", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: {
        title: "Field Isolation Test",
        description: "desc-original",
      },
    });
    expect(createRes.status()).toBe(201);
    const ticket = await createRes.json();

    // Pre-populate custom fields + status via API so we have a known baseline.
    const seedRes = await request.patch(
      `/api/v1/ws/support/tickets/${ticket.id}`,
      {
        data: {
          statusId: "in-progress",
          fields: [
            { fieldId: "priority", value: "high" },
            { fieldId: "category", value: "bug" },
            { fieldId: "reference-url", value: "https://example.com/x" },
          ],
        },
      },
    );
    expect(seedRes.ok()).toBe(true);
    const seeded = await seedRes.json();

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Field Isolation Test")).toBeVisible();

    // Sanity: still not in Edit mode.
    await expect(
      page.getByRole("button", { name: /Save changes/ }),
    ).toHaveCount(0);

    // Inline-edit assignee: pick Bob.
    await page.getByRole("button", { name: /Unassigned/i }).first().click();
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await page.getByRole("option", { name: "Bob" }).click();
    const req = await patchReq;
    expect(JSON.parse(req.postData() ?? "{}")).toEqual({
      assigneeId: "U_BOB",
    });

    // Re-fetch from server: every other field must still match the seed.
    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeId).toBe("U_BOB");
    expect(after.title).toBe("Field Isolation Test");
    expect(after.description).toBe("desc-original");
    expect(after.statusId).toBe("in-progress");
    // Custom fields preserved.
    const fieldMap = Object.fromEntries(
      (after.fields ?? []).map((f: { fieldId: string; value: unknown }) => [
        f.fieldId,
        f.value,
      ]),
    );
    const seededFieldMap = Object.fromEntries(
      (seeded.fields ?? []).map(
        (f: { fieldId: string; value: unknown }) => [f.fieldId, f.value],
      ),
    );
    expect(fieldMap).toEqual(seededFieldMap);
  });

  test("unassign via inline picker sends only assigneeId", async ({
    page,
    request,
  }) => {
    const createRes = await request.post("/api/v1/ws/support/tickets", {
      data: { title: "Unassign Test" },
    });
    const ticket = await createRes.json();
    await request.patch(`/api/v1/ws/support/tickets/${ticket.id}`, {
      data: { assigneeId: "U_ALICE" },
    });

    await page.goto(`/ws/support/tickets/${ticket.id}`);
    await expect(page.getByText("Unassign Test")).toBeVisible();

    // Click the clear (×) button on the assignee picker.
    const clearBtn = page.getByRole("button", { name: "Clear", exact: true });
    await expect(clearBtn).toBeVisible();
    const patchReq = page.waitForRequest(
      (req) =>
        req.method() === "PATCH" &&
        req.url().includes(`/api/v1/ws/support/tickets/${ticket.id}`),
    );
    await clearBtn.click();
    const req = await patchReq;
    expect(JSON.parse(req.postData() ?? "{}")).toEqual({ assigneeId: "" });

    const after = await request
      .get(`/api/v1/ws/support/tickets/${ticket.id}`)
      .then((r) => r.json());
    expect(after.assigneeId ?? "").toBe("");
    expect(after.title).toBe("Unassign Test");
  });
});
