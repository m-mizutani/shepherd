import { test, expect } from "@playwright/test";

test.describe("Authentication (NoAuthn mode)", () => {
  test("login page redirects to workspace list when already authenticated", async ({
    page,
  }) => {
    // In NoAuthn mode, /api/auth/me always succeeds,
    // so /login should redirect to /
    await page.goto("/login");
    await page.waitForURL("/");
    await expect(page.getByText("Workspaces")).toBeVisible();
  });

  test("/api/auth/login redirects to root", async ({ request }) => {
    const res = await request.get("/api/auth/login", {
      maxRedirects: 0,
    });
    expect(res.status()).toBe(307);
    expect(res.headers()["location"]).toBe("/");
  });

  test("/api/auth/me returns user info", async ({ request }) => {
    const meRes = await request.get("/api/auth/me");
    expect(meRes.ok()).toBeTruthy();
    const body = await meRes.json();
    expect(body.sub).toBe("U_E2E");
    expect(body.email).toBeTruthy();
    expect(body.name).toBeTruthy();
  });

  test("logout clears session and shows login page", async ({ page }) => {
    // Login first
    await page.goto("/api/auth/login");
    await page.waitForURL("/");

    // Click sign out
    await page.getByText("Sign out").click();
    await page.waitForURL("/login");
    await expect(page.getByText("Sign in with Slack")).toBeVisible();
  });
});
