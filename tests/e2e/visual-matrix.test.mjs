/**
 * Visual-matrix E2E suite.
 *
 * Drives every primary route at every supported viewport and asserts:
 *   1. The route renders (shell visible, no infinite loaders).
 *   2. No horizontal scroll anywhere — html.scrollWidth <= window.innerWidth.
 *   3. No console errors or page errors during render.
 *   4. A screenshot is captured under `tests/e2e/__screenshots__/matrix/`
 *      via Playwright's `toHaveScreenshot()` snapshot facility (visual diff).
 *
 * Plus dedicated tests for layout invariants per viewport class:
 *   - Mobile (≤760): sidebar hidden until the burger opens its drawer; no bottom bar.
 *   - Tablet portrait (768): sidebar hidden; list and optional detail use two columns.
 *   - Desktop (≥1280): 3 columns visible together (sidebar + list + detail).
 *
 * Run:
 *   bun run test:matrix           # local
 *   CI=1 bun run test:matrix      # full output, 2 workers
 *
 * The matrix runs through its own playwright config
 * (`playwright.matrix.config.mjs`) so it doesn't pollute the default
 * smoke run. Snapshots live in `tests/e2e/__screenshots__/` and are
 * regenerated with `--update-snapshots`.
 */
import { test, expect } from '@playwright/test'
import {
  seedUser,
  seedDomain,
  deliverEmail,
  useTestApi,
  loginAs,
  getToken,
  waitForUiReady,
  expectDuVayIntegrity,
} from './helpers.mjs'

// ── Matrix dimensions ────────────────────────────────────────────────────────

/**
 * Viewport breakpoints the layout responds to. Width is the primary axis;
 * height stays generous so vertical content doesn't get clipped.
 *
 * Picked from the CSS media-queries we ship:
 *   - mobile          ≤ 760
 *   - tablet (port)   761 – 1024
 *   - desktop tight   1025 – 1120
 *   - desktop wide    > 1120
 */
const VIEWPORTS = [
  { name: 'mobile',      width: 375,  height: 812 },
  { name: 'tablet',      width: 768,  height: 1024 },
  { name: 'tablet-land', width: 1024, height: 768 },
  { name: 'desktop',     width: 1280, height: 800 },
  { name: 'wide',        width: 1920, height: 1080 },
]

/**
 * Top-level routes to visit. Every route here must render without auth being
 * required after `withAuth()` has run. We include a representative folder
 * route (drafts) to exercise the dynamic `/folder/:name` page.
 */
const ROUTES = [
  { path: '/inbox',         name: 'inbox' },
  { path: '/folder/drafts', name: 'folder-drafts' },
  { path: '/folder/sent',   name: 'folder-sent' },
  { path: '/folder/archive',name: 'folder-archive' },
  { path: '/compose',       name: 'compose' },
  { path: '/contacts',      name: 'contacts' },
  { path: '/scheduled',     name: 'scheduled' },
  { path: '/settings',      name: 'settings' },
]

// ── Auth fixture ─────────────────────────────────────────────────────────────

/**
 * Seeds a fresh user and primes the page with their JWT + test-API config.
 * Each test gets its own user so the matrix is hermetic — no cross-test
 * dependencies, safe to run with multiple workers.
 *
 * @param {import('@playwright/test').Page} page
 * @returns {Promise<{ email: string, token: string, domains: string[] }>}
 */
async function withAuth(page, fixture = 'default') {
  const stableFixture = String(fixture).toLowerCase().replace(/[^a-z0-9-]+/g, '-')
  const domain = `matrix-${stableFixture}.test`
  const email = `test@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const data = await getToken(email)
  await useTestApi(page)
  await loginAs(page, data.token, data.email, data.role, data.domains)
  return { email, token: data.token, domains: data.domains }
}

/**
 * Attaches listeners that collect console errors and page errors into the
 * supplied array. Errors-by-reference is the idiomatic Playwright pattern
 * for asserting "no errors during the whole test".
 *
 * @param {import('@playwright/test').Page} page
 * @param {string[]} sink
 */
function captureErrors(page, sink) {
  page.on('pageerror', err => sink.push(`pageerror: ${err.message}`))
  page.on('console', msg => {
    if (msg.type() !== 'error') return
    const text = msg.text()
    // Filter known noisy warnings that aren't actionable in the SPA shell.
    if (/Failed to load resource.*\/sw\.js/.test(text)) return
    if (/favicon\.ico/.test(text)) return
    sink.push(`console: ${text}`)
  })
}

/**
 * Read horizontal-overflow stats from the page so we can assert no h-scroll.
 * Returns the absolute overage (>0 means horizontal scroll). 1px slack is
 * tolerated because sub-pixel rounding on retina viewports can produce
 * 0.5px overages that aren't visually meaningful.
 *
 * @param {import('@playwright/test').Page} page
 */
async function measureHorizontalOverflow(page) {
  return page.evaluate(() => {
    const doc = document.documentElement
    return {
      scrollWidth: doc.scrollWidth,
      innerWidth: window.innerWidth,
      overage: Math.max(0, doc.scrollWidth - window.innerWidth),
    }
  })
}

/**
 * Wait until route-level asynchronous placeholders have left the rendered
 * page. A shell-only readiness check is too early for deterministic visual
 * snapshots because inbox, settings, and scheduled data load after mount.
 *
 * @param {import('@playwright/test').Page} page
 */
async function waitForRouteData(page) {
  await expect(
    page.locator('.inbox-empty').filter({ hasText: /Loading/ }),
  ).toHaveCount(0, { timeout: 20_000 })
}

// ── Matrix: snapshot + no-scroll + no-error per (viewport × route) ───────────

for (const vp of VIEWPORTS) {
  test.describe(`@${vp.name} (${vp.width}×${vp.height})`, () => {
    for (const route of ROUTES) {
      test(`${route.name} renders without scroll/errors`, async ({ page }) => {
        const errors = []
        captureErrors(page, errors)

        await page.setViewportSize({ width: vp.width, height: vp.height })
        await withAuth(page, `${vp.name}-${route.name}`)
        await page.goto(route.path)

        await waitForUiReady(page)
        await waitForRouteData(page)
        await expectDuVayIntegrity(page, expect)

        // Visual snapshot (creates baseline on first run, diff on subsequent)
        await expect(page).toHaveScreenshot(`${vp.name}-${route.name}.png`, {
          fullPage: true,
          // Mask animation-prone regions (none yet, placeholder for future).
          maxDiffPixelRatio: 0.02,
        })

        // No horizontal scroll on either the document or .app-shell
        const overflow = await measureHorizontalOverflow(page)
        expect(
          overflow.overage,
          `Horizontal scroll on ${route.path} @${vp.name}: ` +
          `scrollWidth=${overflow.scrollWidth} innerWidth=${overflow.innerWidth}`
        ).toBeLessThanOrEqual(1)

        // No console / page errors during render
        expect(errors, `Errors during render @${vp.name}${route.path}`).toEqual([])
      })
    }
  })
}

// ── Layout invariants per viewport class ─────────────────────────────────────

test('mobile (375×812): burger opens the sidebar and no bottom bar exists', async ({ page }) => {
  await page.setViewportSize({ width: 375, height: 812 })
  await withAuth(page, 'mobile-navigation')
  await page.goto('/inbox')
  await expect(page.locator('.app-brand-link')).toBeVisible()
  await expect(page.locator('.app-sidebar')).toBeHidden()
  await expect(page.locator('.mobile-nav,.mobile-bottom-nav')).toHaveCount(0)
  await page.getByRole('button', { name: 'Open navigation menu' }).click()
  await expect(page.locator('.app-sidebar')).toBeVisible()
})

test('tablet (768×1024) no-selection: sidebar and detail hidden, list visible', async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 1024 })
  await withAuth(page, 'tablet-no-selection')
  await page.goto('/inbox')
  await expect(page.locator('.app-brand-link')).toBeVisible()
  await expect(page.locator('.app-sidebar')).toBeHidden()
  await expect(page.locator('.mail-list-panel')).toBeVisible()
  await expect(page.locator('.mail-detail-panel')).toBeHidden()
})

test('tablet (768×1024) with-selection: list+detail visible without redundant Back', async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 1024 })
  const { email, token, domains } = await withAuth(page, 'tablet-selection')
  await seedDomain(token, domains[0])
  await deliverEmail({
    recipient: email,
    sender: 'someone@vendor.com',
    subject: 'matrix-tablet-select',
    body: 'verify sidebar collapses on tablet when an email is selected',
    messageId: `matrix-tablet-${Date.now()}`,
  })
  await page.goto('/inbox')
  await page.locator('.email-row').first().waitFor({ timeout: 15_000 })

  // Click an email
  await page.locator('.email-row').first().click({ position: { x: 80, y: 24 } })
  await page.locator('.email-body').waitFor({ timeout: 15_000 })

  await expect(page.locator('.app-sidebar')).toBeHidden()
  await expect(page.locator('.mail-list-panel')).toBeVisible()
  await expect(page.locator('.mail-detail-panel')).toBeVisible()
  await expect(page.locator('.email-back-btn')).toBeHidden()
})

test('desktop (1280×800): all three panes visible together', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 })
  await withAuth(page, 'desktop-three-pane')
  await page.goto('/inbox')
  await expect(page.locator('.app-brand-link')).toBeVisible()
  await expect(page.locator('.app-sidebar')).toBeVisible()
  await expect(page.locator('.mail-list-panel')).toBeVisible()
  await expect(page.locator('.mail-detail-panel')).toBeVisible()
})

// ── Compact folder navigation stays directly scrollable ─────────────────────

test('folder tabs (tablet): native horizontal scrolling replaces More', async ({ page }) => {
  await page.setViewportSize({ width: 768, height: 1024 })
  await withAuth(page, 'tablet-folder-tabs')
  await page.goto('/inbox')
  await page.locator('.folder-tabs').waitFor({ timeout: 15_000 })

  await expect(page.locator('.folder-tabs .overflow-more-btn')).toHaveCount(0)
  const metrics = await page.locator('.folder-tabs').evaluate(el => ({
    overflowX: getComputedStyle(el).overflowX,
    before: el.scrollLeft,
    max: el.scrollWidth - el.clientWidth,
  }))
  expect(['auto', 'scroll']).toContain(metrics.overflowX)
  if (metrics.max > 0) {
    await page.locator('.folder-tabs').evaluate(el => { el.scrollLeft = el.scrollWidth })
    expect(await page.locator('.folder-tabs').evaluate(el => el.scrollLeft)).toBeGreaterThan(metrics.before)
  }
})
