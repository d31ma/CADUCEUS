import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  testMatch: '**/*.test.mjs',
  testIgnore: ['**/visual-matrix.test.mjs', '**/accessibility.test.mjs'],
  fullyParallel: false,
  timeout: 60_000,
  workers: process.env.CI ? 2 : 1,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: 'http://localhost:19875',
    bypassCSP: true,
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chrome', use: { ...devices['Desktop Chrome'], channel: 'chrome', permissions: ['local-network-access'] } },
  ],
  // One origin: `ty serve` runs the Go server and the client together,
  // with the test seeding routes on (scripts/e2e-preview.mjs). A cold start
  // compiles every Go handler, so allow it the suites' startup budget.
  webServer: {
    command: 'bun scripts/e2e-preview.mjs',
    url: 'http://localhost:19875',
    reuseExistingServer: false,
    timeout: 900_000,
  },
})
