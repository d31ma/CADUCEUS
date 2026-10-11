/**
 * Playwright config for the visual-matrix suite.
 *
 * Inherits the same preview + API web-servers as the smoke run, but matches
 * only `visual-matrix.test.mjs`. Snapshots live under
 * `tests/e2e/__screenshots__/`. Regenerate baselines with:
 *
 *   bun run test:matrix --update-snapshots
 *
 * Kept as a separate config so the default `bun run test:e2e` (smoke) stays
 * fast and the matrix can be opted into for visual-regression / pre-release
 * verification.
 */
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  testMatch: '**/visual-matrix.test.mjs',
  // Store screenshot baselines in a single flat `__screenshots__/` directory
  // instead of Playwright's default `<test-file>-snapshots/` so the matrix
  // baselines are easy to browse / diff in PRs. The template uses {arg} which
  // is the snapshot name we pass to `toHaveScreenshot('xyz.png')`.
  snapshotPathTemplate: 'tests/e2e/__screenshots__/{arg}{ext}',
  // Keep the visual baseline serial. Parallel browser contexts each retain a
  // long-lived inbox event stream and can make Playwright's worker teardown
  // hang after every assertion has passed.
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  // Generous timeout for route data and deterministic DuVay upgrade settling.
  timeout: 60_000,
  expect: {
    toHaveScreenshot: {
      // 2% pixel-difference tolerance handles antialiasing across runs.
      // Tighten this if you want stricter visual regressions.
      maxDiffPixelRatio: 0.02,
    },
  },
  use: {
    baseURL: 'http://localhost:19875',
    bypassCSP: true,
    trace: 'on-first-retry',
    // Capture screenshots even on success so the snapshots dir stays current.
    screenshot: 'only-on-failure',
  },
  projects: [
    { name: 'chrome', use: { ...devices['Desktop Chrome'], channel: 'chrome' } },
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
