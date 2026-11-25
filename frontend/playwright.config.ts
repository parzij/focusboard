import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  workers: 1,
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:5173",
    browserName: "chromium",
    viewport: { width: 1440, height: 900 },
  },
  webServer: {
    command: "node node_modules/vite/bin/vite.js",
    url: "http://127.0.0.1:5173/api/v1/ready",
    reuseExistingServer: !process.env.CI,
    timeout: 30000,
  },
});
