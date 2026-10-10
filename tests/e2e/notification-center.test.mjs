import { test, expect } from '@playwright/test'
import { captureUiErrors, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

test('prioritizes connection health in the responsive notification center', async ({ page }) => {
  const errors = captureUiErrors(page)
  const domain = `notifications-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)

  await page.evaluate(() => {
    Object.defineProperty(navigator, 'onLine', { configurable: true, get: () => false })
    window.dispatchEvent(new Event('offline'))
  })
  const sidebarNotice = page.locator('.sidebar-notification[data-notification-id="offline"]')
  await expect(sidebarNotice).toContainText('CADUCEUS is offline')
  await expect(sidebarNotice).toHaveAttribute('data-priority', '5')

  const trigger = page.locator('.app-notification-trigger')
  await expect(trigger).toHaveAttribute('aria-label', /Notifications, \d+ active/)
  await trigger.click()
  const panel = page.getByRole('dialog', { name: 'Notifications' })
  const offline = panel.locator('[data-notification-id="offline"]')
  await expect(offline).toBeVisible()
  await expect(offline.getByRole('button', { name: 'Dismiss' })).toHaveCount(0)

  await page.setViewportSize({ width: 360, height: 640 })
  await expect(panel).toHaveCSS('width', '360px')
  await panel.getByRole('button', { name: 'Close notifications' }).click()

  await page.evaluate(() => {
    Object.defineProperty(navigator, 'onLine', { configurable: true, get: () => true })
    window.dispatchEvent(new Event('online'))
  })
  await expect(sidebarNotice).toHaveCount(0)
  expect(errors).toEqual([])
})
