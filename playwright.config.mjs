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
    // The production app uses same-origin APIs. E2E isolates its API on a
    // dedicated port, so bypass CSP here while the CORS proxy remains active.
    bypassCSP: true,
    trace: 'on-first-retry',
  },
  projects: [
    { name: 'chrome', use: { ...devices['Desktop Chrome'], channel: 'chrome', permissions: ['local-network-access'] } },
  ],
  webServer: [
    {
      // Static preview of the bundled app
      command: 'bun scripts/e2e-preview.mjs',
      url: 'http://localhost:19875',
      reuseExistingServer: false,
      timeout: 300_000,
    },
    {
      // Real route handlers via the transitional Bun test adapter, backed by
      // the Go FYLO owner and temporary released FYLO/SESAME runtimes.
      command: 'bun scripts/e2e-api.mjs',
      url: 'http://localhost:19876',
      reuseExistingServer: false,
      timeout: 300_000,
    },
  ],
})
