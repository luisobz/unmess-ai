import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests",
  testMatch: "**/*.spec.js",
  fullyParallel: false,
  workers: 1,
  use: {
    baseURL: "http://127.0.0.1:48339",
    locale: "es-ES",
    channel: process.env.UNMESSAI_TEST_BROWSER || undefined,
    viewport: { width: 1440, height: 1000 },
  },
  webServer: {
    command: "go run ./tests/fixture",
    url: "http://127.0.0.1:48339",
    reuseExistingServer: false,
    timeout: 60000,
  },
  reporter: "list",
});
