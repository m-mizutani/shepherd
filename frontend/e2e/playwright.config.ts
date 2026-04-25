import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://localhost:18923",
    screenshot: "only-on-failure",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
    },
  ],
  webServer: {
    command:
      "cd ../.. && go run . serve --no-authn U_E2E --repository-backend memory --config examples/config.toml --addr :18923 --llm-provider openai --llm-openai-api-key sk-e2e-dummy --agent-storage-fs-dir /tmp/shepherd-e2e-agent-store",
    url: "http://localhost:18923/api/v1/health",
    reuseExistingServer: false,
    timeout: 60000,
  },
});
