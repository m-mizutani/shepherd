import { test, expect, type APIRequestContext } from "@playwright/test";

// All tests share one memory-backed server (Playwright runs with workers:1
// and the spec file's webServer is started once per session). Since there is
// no "reset prompt" API, every test reads the current version through the
// API first and asserts deltas relative to that baseline rather than absolute
// version numbers — this keeps tests independent of execution order.
//
// The very first test in this file additionally asserts the embedded-default
// behavior, which is only meaningful before any other prompts test has run.
// Playwright preserves declaration order within a spec file.

async function getDetail(request: APIRequestContext) {
  const res = await request.get("/api/v1/ws/support/prompts/triage");
  expect(res.status()).toBe(200);
  return res.json() as Promise<{
    id: string;
    content: string;
    version: number;
    isOverride: boolean;
    defaultContent: string;
    variables: string[];
  }>;
}

async function getHistory(request: APIRequestContext) {
  const res = await request.get(
    "/api/v1/ws/support/prompts/triage/history",
  );
  expect(res.status()).toBe(200);
  return (await res.json()).versions as Array<{
    version: number;
    content: string;
    current: boolean;
    updatedBy?: { name: string } | null;
  }>;
}

test.describe("Prompts settings", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/api/auth/login");
    await page.waitForURL("/");
  });

  test("first load shows the embedded default and version 0", async ({
    page,
    request,
  }) => {
    // This test is meaningful only before any save has happened.
    const detail = await getDetail(request);
    test.skip(
      detail.version !== 0,
      "Default-state assertions require a fresh server (version === 0).",
    );

    await page.goto("/ws/support/settings/prompts");

    await expect(
      page.getByRole("heading", { name: "Prompts" }),
    ).toBeVisible();

    const triageCard = page
      .getByRole("button", { name: /Triage/ })
      .filter({ hasText: "Classify" });
    await expect(triageCard).toBeVisible();
    await expect(triageCard.getByText("Not configured")).toBeVisible();

    // Effective content equals the embedded default when no override exists.
    expect(detail.isOverride).toBe(false);
    expect(detail.content).toBe(detail.defaultContent);
    expect(detail.content).toContain("{{ .Title }}");
    expect(detail.variables).toEqual(
      expect.arrayContaining(["Title", "Description", "Reporter"]),
    );

    await expect(page.locator("textarea")).toHaveValue(detail.content);
  });

  test("saving from the editor advances the version by one", async ({
    page,
    request,
  }) => {
    const before = await getDetail(request);

    await page.goto("/ws/support/settings/prompts");

    const newContent = `# E2E saved at ${Date.now()} for {{ .Title }}\nReporter: {{ .Reporter }}`;
    const textarea = page.locator("textarea");
    await textarea.fill(newContent);

    await expect(page.getByText("Unsaved changes")).toBeVisible();
    await page.getByRole("button", { name: "Save", exact: true }).click();
    await expect(page.getByText("All changes saved")).toBeVisible({
      timeout: 5000,
    });

    const after = await getDetail(request);
    expect(after.version).toBe(before.version + 1);
    expect(after.isOverride).toBe(true);
    expect(after.content).toBe(newContent);

    const history = await getHistory(request);
    expect(history.length).toBe(after.version);
    const latest = history[history.length - 1];
    expect(latest.version).toBe(after.version);
    expect(latest.current).toBe(true);
    expect(latest.content).toBe(newContent);
    // Author is the no-authn dev user (sub=U_E2E).
    expect(latest.updatedBy?.name).toBeTruthy();
  });

  test("invalid template surfaces a 422 reason and does not persist", async ({
    page,
    request,
  }) => {
    const before = await getDetail(request);

    await page.goto("/ws/support/settings/prompts");

    // {{ .NonExistent }} parses but fails Execute under missingkey=error.
    await page.locator("textarea").fill("hi {{ .NonExistent }}");
    const responsePromise = page.waitForResponse(
      (res) =>
        res.url().endsWith("/api/v1/ws/support/prompts/triage") &&
        res.request().method() === "PUT",
    );
    await page.getByRole("button", { name: "Save", exact: true }).click();

    const response = await responsePromise;
    expect(response.status()).toBe(422);
    const body = await response.json();
    expect(body.error).toBe("invalid_template");
    expect(body.reason).toContain("NonExistent");

    const banner = page.getByText(/Template error/);
    await expect(banner).toBeVisible({ timeout: 5000 });
    await expect(banner).toContainText("NonExistent");

    const after = await getDetail(request);
    expect(after.version).toBe(before.version);
    expect(after.content).toBe(before.content);
  });

  test("stale version surfaces a 409 with reload affordance", async ({
    page,
    request,
  }) => {
    // Capture the version the editor will start with, then race a winning
    // save through the API so the editor's intended next version is stale.
    const baseline = await getDetail(request);

    await page.goto("/ws/support/settings/prompts");
    // Wait until the page actually loaded the editor with the baseline; this
    // is what locks the editor's "next version = baseline.version + 1" in
    // React state.
    await expect(page.locator("textarea")).toHaveValue(baseline.content);

    const winnerContent = `winner-${Date.now()} {{ .Title }}`;
    const winnerRes = await request.put(
      "/api/v1/ws/support/prompts/triage",
      {
        data: {
          content: winnerContent,
          version: baseline.version + 1,
        },
      },
    );
    expect(winnerRes.status()).toBe(200);

    // Now the browser submits with the now-stale next version. Capture the
    // PUT response so we can verify the wire-level contract (409 + the
    // version_conflict body), not just the UI banner.
    await page
      .locator("textarea")
      .fill(`loser-${Date.now()} {{ .Title }}`);
    const responsePromise = page.waitForResponse(
      (res) =>
        res.url().endsWith("/api/v1/ws/support/prompts/triage") &&
        res.request().method() === "PUT",
    );
    await page.getByRole("button", { name: "Save", exact: true }).click();

    const response = await responsePromise;
    expect(response.status()).toBe(409);
    const body = await response.json();
    expect(body.error).toBe("version_conflict");
    expect(body.currentVersion).toBe(baseline.version + 1);

    // The UI surfaces it as a banner with a reload affordance.
    await expect(
      page.getByText(/Someone else just saved version/),
    ).toBeVisible({ timeout: 5000 });
    await expect(
      page.getByRole("button", { name: /Discard my edits and reload/ }),
    ).toBeVisible();

    // Winner's content is the one that survived.
    const detail = await getDetail(request);
    expect(detail.content).toBe(winnerContent);
    expect(detail.version).toBe(baseline.version + 1);
  });

  test("two browser contexts editing concurrently: second saver gets a conflict", async ({
    browser,
    request,
  }) => {
    // Simulate two users (A and B) both opening the editor with the same
    // baseline. This is the canonical optimistic-locking scenario the WebUI
    // must defend against: A saves first, B's later save MUST be rejected
    // even though both started from the same server state.
    const ctxA = await browser.newContext();
    const ctxB = await browser.newContext();
    const pageA = await ctxA.newPage();
    const pageB = await ctxB.newPage();
    try {
      await pageA.goto("/api/auth/login");
      await pageA.waitForURL("/");
      await pageB.goto("/api/auth/login");
      await pageB.waitForURL("/");

      // Snapshot the server-side baseline both users will load.
      const baseline = await getDetail(request);

      // Both users open the editor concurrently and read the same content.
      await pageA.goto("/ws/support/settings/prompts");
      await pageB.goto("/ws/support/settings/prompts");
      await expect(pageA.locator("textarea")).toHaveValue(baseline.content);
      await expect(pageB.locator("textarea")).toHaveValue(baseline.content);

      // User A edits and saves first.
      const aContent = `userA-${Date.now()} for {{ .Title }}`;
      await pageA.locator("textarea").fill(aContent);
      await pageA.getByRole("button", { name: "Save", exact: true }).click();
      await expect(pageA.getByText("All changes saved")).toBeVisible({
        timeout: 5000,
      });

      // User B, still on the original baseline, now tries to save. Capture
      // the PUT response to verify the server actually returned 409 with the
      // expected body — not just that the UI happened to render a banner.
      const bContent = `userB-${Date.now()} for {{ .Title }}`;
      await pageB.locator("textarea").fill(bContent);
      const bResponsePromise = pageB.waitForResponse(
        (res) =>
          res.url().endsWith("/api/v1/ws/support/prompts/triage") &&
          res.request().method() === "PUT",
      );
      await pageB.getByRole("button", { name: "Save", exact: true }).click();

      const bResponse = await bResponsePromise;
      expect(bResponse.status()).toBe(409);
      const bBody = await bResponse.json();
      expect(bBody.error).toBe("version_conflict");
      expect(bBody.currentVersion).toBe(baseline.version + 1);

      // And B must see the conflict banner — silent overwrite is a bug.
      await expect(
        pageB.getByText(/Someone else just saved version/),
      ).toBeVisible({ timeout: 5000 });
      await expect(
        pageB.getByRole("button", { name: /Discard my edits and reload/ }),
      ).toBeVisible();

      // The server-side content is A's, not B's. B's save did not persist.
      const finalDetail = await getDetail(request);
      expect(finalDetail.content).toBe(aContent);
      expect(finalDetail.version).toBe(baseline.version + 1);
    } finally {
      await ctxA.close();
      await ctxB.close();
    }
  });

  test("history overlay restores a prior version as the next version", async ({
    page,
    request,
  }) => {
    // Seed two known versions on top of whatever is already there.
    const baseline = await getDetail(request);
    const restoreTarget = `restore-target-${Date.now()} {{ .Title }}`;
    const v1Save = await request.put(
      "/api/v1/ws/support/prompts/triage",
      {
        data: {
          content: restoreTarget,
          version: baseline.version + 1,
        },
      },
    );
    expect(v1Save.status()).toBe(200);
    const v1 = (await v1Save.json()) as { version: number };

    const v2Save = await request.put(
      "/api/v1/ws/support/prompts/triage",
      {
        data: {
          content: `intermediate-${Date.now()} {{ .Title }}`,
          version: v1.version + 1,
        },
      },
    );
    expect(v2Save.status()).toBe(200);
    const v2 = (await v2Save.json()) as { version: number };

    await page.goto("/ws/support/settings/prompts");
    await page.getByRole("button", { name: "History" }).click();

    // Wait for the overlay to render. The header is a unique anchor.
    await expect(
      page.getByText(/Triage prompt — History/),
    ).toBeVisible();

    // Both seeded versions appear in the timeline. We search at page scope:
    // once the overlay is open, `v{n}` strings only appear inside it.
    await expect(
      page.getByRole("button", { name: new RegExp(`^v${v2.version}\\b`) }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: new RegExp(`^v${v1.version}\\b`) }),
    ).toBeVisible();
    await expect(page.getByText("Live").first()).toBeVisible();

    // Pick v1 in the timeline to set it as the comparison baseline,
    // then restore.
    await page
      .getByRole("button", { name: new RegExp(`^v${v1.version}\\b`) })
      .click();
    await page
      .getByRole("button", { name: new RegExp(`Restore v${v1.version}`) })
      .click();

    // History gets a new entry with v1's content as the latest version.
    await expect
      .poll(async () => (await getHistory(request)).length)
      .toBe(v2.version + 1);

    const detail = await getDetail(request);
    expect(detail.version).toBe(v2.version + 1);
    expect(detail.content).toBe(restoreTarget);
  });
});
