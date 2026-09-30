import { defineConfig } from "@playwright/test";

const deployedURL = process.env.TRIPPY_E2E_URL;

export default defineConfig({
  testDir: "./tests",
  outputDir: "/tmp/trippy-playwright-results",
  use: {
    baseURL: deployedURL ?? "http://127.0.0.1:4173",
    launchOptions: process.env.PLAYWRIGHT_CHROMIUM_PATH
      ? { executablePath: process.env.PLAYWRIGHT_CHROMIUM_PATH }
      : {},
    trace: "retain-on-failure",
  },
  webServer: deployedURL ? undefined : {
    command: "npm run dev -- --host 127.0.0.1 --port 4173 --strictPort",
    url: "http://127.0.0.1:4173",
    reuseExistingServer: !process.env.CI,
  },
});
