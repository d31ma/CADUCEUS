import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('checks grammar locally, applies a correction, ignores a rule, and never serializes highlights', async ({ page }) => {
  const domain = `grammar-${Date.now()}.test`
  const mailbox = `alice@${domain}`
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const alice = await getToken(mailbox)
  await seedDomain(alice.token, domain)

  await page.setViewportSize({ width: 1280, height: 900 })
  await useTestApi(page)
  await loginAs(page, alice.token, alice.email, alice.role, alice.domains)
  const errors = captureUiErrors(page)
  await page.goto('/compose')
  await waitForUiReady(page)

  const toggle = page.getByRole('button', { name: 'Enable grammar check' })
  await expect(toggle).toHaveAttribute('aria-pressed', 'false')
  const enabled = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.grammarCheckEnabled === true)
  await toggle.click()
  await enabled
  await expect(page.locator('[data-grammar-toggle]')).toHaveAttribute('aria-pressed', 'true')

  const languageSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().postDataJSON()?.grammarCheckLanguage === 'en-GB')
  await page.getByLabel('Grammar check language').selectOption('en-GB')
  await languageSaved

  const checked = page.waitForResponse(response =>
    response.url().endsWith('/grammar/check') && response.request().method() === 'POST')
  const editor = page.locator('[data-compose-editor]')
  await editor.fill('i think she are ready.')
  await checked
  const panel = page.getByRole('region', { name: 'Grammar check' })
  await expect(panel.getByText('2 grammar issues.')).toBeVisible()
  await expect(panel.locator('.grammar-issue')).toHaveCount(2)
  await expect.poll(() => page.evaluate(() => CSS.highlights?.get('caduceus-grammar-errors')?.size || 0)).toBe(2)

  const agreement = panel.locator('.grammar-issue').filter({ hasText: 'she are' })
  const rechecked = page.waitForResponse(response => response.url().endsWith('/grammar/check'))
  await agreement.getByRole('button', { name: 'she is' }).click()
  await rechecked
  await expect(editor).toHaveText('i think she is ready.')
  await expect(panel.getByText('1 grammar issue.')).toBeVisible()

  const ignored = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().postDataJSON()?.grammarCheckDisabledRules?.includes('LOWERCASE_I'))
  await panel.locator('.grammar-issue').filter({ hasText: '“i”' }).getByRole('button', { name: 'Ignore rule' }).click()
  await ignored
  await expect(panel.getByText('No grammar issues.')).toBeVisible()
  expect(await editor.evaluate(element => element.innerHTML)).not.toMatch(/grammar|highlight/i)

  const preferences = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${alice.token}` },
  }).then(response => response.json())
  expect(preferences).toMatchObject({
    grammarCheckEnabled: true,
    grammarCheckLanguage: 'en-GB',
    grammarCheckDisabledRules: ['LOWERCASE_I'],
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await expect(panel).toBeVisible()
  await expect.poll(async () => (await panel.boundingBox())?.width ?? Infinity).toBeLessThanOrEqual(390)
  expect(errors).toEqual([])
})
