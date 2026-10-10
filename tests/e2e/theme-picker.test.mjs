import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  expectDuVayIntegrity,
  getToken,
  loginAs,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

const PRIMARY_ROUTES = [
  '/inbox', '/folder/drafts', '/folder/sent', '/folder/archive', '/folder/spam',
  '/folder/trash', '/scheduled', '/compose', '/contacts', '/calendar', '/team', '/settings',
]

function luminance([r, g, b]) {
  const channels = [r, g, b].map(value => {
    const normalized = value / 255
    return normalized <= 0.04045 ? normalized / 12.92 : ((normalized + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
}

function contrast(foreground, background) {
  const light = Math.max(luminance(foreground), luminance(background))
  const dark = Math.min(luminance(foreground), luminance(background))
  return (light + 0.05) / (dark + 0.05)
}

test('selects, previews, validates, and persists every built-in theme', async ({ page }) => {
  const domain = `themes-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await page.emulateMedia({ colorScheme: 'light' })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)
  await expect(page.getByRole('button', { name: 'Use Automatic theme' })).toHaveAttribute('aria-pressed', 'true')
  await expect(page.locator('[data-automatic-theme="light"]')).toHaveValue('light')
  await expect(page.locator('[data-automatic-theme="dark"]')).toHaveValue('dark')

  const darkSlotSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.themePreferences?.darkTheme === 'darkside')
  await page.locator('[data-automatic-theme="dark"]').selectOption('darkside')
  expect((await darkSlotSaved).status()).toBe(200)
  await page.emulateMedia({ colorScheme: 'dark' })
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'darkside')
  await expect(page.locator('html')).toHaveAttribute('data-theme-choice', 'automatic')

  const themeColors = new Set()
  for (const theme of ['light', 'dark', 'darkside', 'less-is-more', 'taiga', 'ubuntu']) {
    const saved = page.waitForResponse(response =>
      response.url().endsWith('/users/me/preferences') &&
      response.request().method() === 'PUT' &&
      response.request().postDataJSON()?.themePreferences?.activeTheme === theme)
    await page.getByRole('button', { name: `Use ${theme === 'less-is-more' ? 'Less Is More' : theme[0].toUpperCase() + theme.slice(1)} theme` }).click()
    expect((await saved).status()).toBe(200)
    await expect(page.locator('html')).toHaveAttribute('data-ui-theme', theme)
    const colors = await page.evaluate(() => {
      const style = getComputedStyle(document.body)
      const parse = value => (value.match(/\d+(?:\.\d+)?/g) || []).slice(0, 3).map(Number)
      return {
        foreground: parse(style.color),
        background: parse(style.backgroundColor),
        sidebar: getComputedStyle(document.querySelector('.app-sidebar')).backgroundColor,
      }
    })
    expect(colors.foreground).toHaveLength(3)
    expect(colors.background).toHaveLength(3)
    expect(contrast(colors.foreground, colors.background), `${theme} body contrast`).toBeGreaterThanOrEqual(4.5)
    themeColors.add(colors.sidebar)
  }
  expect(themeColors.size).toBeGreaterThanOrEqual(4)

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted.themePreferences).toEqual({
    activeTheme: 'ubuntu', lightTheme: 'light', darkTheme: 'darkside',
  })

  await page.reload()
  await waitForUiReady(page)
  await expect(page.locator('html')).toHaveAttribute('data-ui-theme', 'ubuntu')
  await expect(page.getByRole('button', { name: 'Use Ubuntu theme' })).toHaveAttribute('aria-pressed', 'true')

  await page.setViewportSize({ width: 320, height: 568 })
  const dimensions = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth }))
  expect(dimensions.document - dimensions.viewport).toBeLessThanOrEqual(1)
  expect(errors).toEqual([])
})

test('light and dark themes preserve every primary route and shell boundary', async ({ page }) => {
  const domain = `theme-matrix-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  for (const theme of ['light', 'dark']) {
    const response = await fetch(`${API}/users/me/preferences`, {
      method: 'PUT',
      headers: { Authorization: `Bearer ${account.token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        themePreferences: { activeTheme: theme, lightTheme: 'light', darkTheme: 'dark' },
      }),
    })
    expect(response.status).toBe(200)

    for (const route of PRIMARY_ROUTES) {
      await page.goto(route)
      await waitForUiReady(page)
      await expect(page.locator('html')).toHaveAttribute('data-ui-theme', theme)
      await expectDuVayIntegrity(page, expect)
      const metrics = await page.evaluate(() => ({
        viewport: innerWidth,
        document: document.documentElement.scrollWidth,
        mode: getComputedStyle(document.documentElement).colorScheme,
      }))
      expect(metrics.document - metrics.viewport, `${theme} ${route} overflow`).toBeLessThanOrEqual(1)
      expect(metrics.mode).toContain(theme)
    }
  }
  expect(errors).toEqual([])
})
