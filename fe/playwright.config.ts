import { defineConfig, devices } from "@playwright/test";
import { env } from "node:process";

export default defineConfig({
  testDir: "./tests",
  fullyParallel: false,
  // Each page owns a Go runtime and a WebGL2 context. Serial execution avoids
  // GPU-process exhaustion and makes context-loss tests deterministic in CI.
  workers: 1,
  forbidOnly: Boolean(env.CI),
  retries: env.CI ? 2 : 0,
  reporter: env.CI ? "github" : "list",
  use: {
    baseURL: "http://127.0.0.1:4173",
    trace: "on-first-retry",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "npm run dev -- --host 127.0.0.1 --port 4173",
    url: "http://127.0.0.1:4173",
    reuseExistingServer: !env.CI,
    timeout: 120_000,
  },
});
