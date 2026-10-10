import { expect, test } from '@playwright/test'
import {
  API,
  captureUiErrors,
  deliverEmail,
  expectDuVayIntegrity,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('persists localized shell catalogs and preserves an RTL responsive layout', async ({ page }) => {
  const domain = `locale-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  await deliverEmail({
    recipient: account.email,
    sender: 'localization-safety@example.net',
    subject: 'Archive',
    body: 'Save',
  })
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)
  await expect(page.locator('select[data-interface-locale]')).toHaveValue('auto')

  const frenchSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.interfaceLocale === 'fr')
  await page.locator('select[data-interface-locale]').selectOption('fr')
  expect((await frenchSaved).status()).toBe(200)
  await expect(page.locator('html')).toHaveAttribute('lang', 'fr')
  await expect(page.locator('html')).toHaveAttribute('dir', 'ltr')
  await expect(page.locator('select[data-interface-locale]')).toHaveValue('fr')
  await expect(page.getByRole('link', { name: /^Toutes les boîtes(?:\s+\d+)?$/ })).toBeVisible()
  await expect(page.getByText('Langue et région', { exact: true })).toBeVisible()
  await expect(page.locator('html')).toHaveAttribute('data-interface-catalog-ready', '')

  await page.goto('/inbox')
  await waitForUiReady(page)
  const localizedSafetyMessage = page.locator('.email-row', { hasText: 'Archive' })
  await expect(localizedSafetyMessage.locator('.email-subject-text', { hasText: 'Archive' })).toBeVisible()
  await expect(localizedSafetyMessage.locator('.email-sender')).toContainText('localization-safety@example.net')
  await expect(page.locator('.email-subject-text', { hasText: 'Archives' })).toHaveCount(0)
  await localizedSafetyMessage.locator('[data-email-open]').click({ position: { x: 120, y: 32 } })
  await expect(page.locator('.conversation-message-body,.email-body').filter({ hasText: 'Save' }).first()).toBeVisible()

  await page.reload()
  await waitForUiReady(page)
  await expect(page.locator('html')).toHaveAttribute('lang', 'fr')
  await page.goto('/settings')
  await waitForUiReady(page)
  await expect(page.locator('select[data-interface-locale]')).toHaveValue('fr')

  await page.goto('/calendar')
  await waitForUiReady(page)
  const frenchMonth = new Intl.DateTimeFormat('fr', { month: 'long', year: 'numeric' }).format(new Date())
  await expect(page.locator('.calendar-title')).toHaveText(frenchMonth)

  await page.goto('/settings')
  await waitForUiReady(page)

  const arabicSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.interfaceLocale === 'ar')
  await page.locator('select[data-interface-locale]').selectOption('ar')
  expect((await arabicSaved).status()).toBe(200)
  await expect(page.locator('html')).toHaveAttribute('lang', 'ar')
  await expect(page.locator('html')).toHaveAttribute('dir', 'rtl')
  await expect(page.getByRole('link', { name: /^كل صناديق الوارد(?:\s+\d+)?$/ })).toBeVisible()

  await page.setViewportSize({ width: 320, height: 568 })
  await expectDuVayIntegrity(page, expect)
  const metrics = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
    locale: document.documentElement.dataset.interfaceLocale,
    direction: getComputedStyle(document.documentElement).direction,
  }))
  expect(metrics.document - metrics.viewport).toBeLessThanOrEqual(1)
  expect(metrics).toMatchObject({ locale: 'ar', direction: 'rtl' })

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted.interfaceLocale).toBe('ar')
  expect(errors).toEqual([])
})
