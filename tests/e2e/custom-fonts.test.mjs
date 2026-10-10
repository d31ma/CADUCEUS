import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  getToken,
  loginAs,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('persists separate local interface and composer fonts across navigation and reload', async ({ page }) => {
  const domain = `fonts-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)

  const interfaceSelect = page.locator('[data-interface-font]')
  const composerSelect = page.locator('[data-composer-font]')
  await expect(interfaceSelect).toHaveValue('plex-sans')
  await expect(composerSelect).toHaveValue('interface')

  const interfaceSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.interfaceFont === 'serif')
  await interfaceSelect.selectOption('serif')
  expect((await interfaceSaved).status()).toBe(200)
  await expect.poll(() => page.evaluate(() => getComputedStyle(document.body).fontFamily)).toContain('Charter')

  const composerSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.composerFont === 'monospace')
  await composerSelect.selectOption('monospace')
  expect((await composerSaved).status()).toBe(200)

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted).toMatchObject({ interfaceFont: 'serif', composerFont: 'monospace' })

  await page.goto('/compose')
  await waitForUiReady(page)
  await expect.poll(() => page.locator('[data-compose-editor]').evaluate(element => getComputedStyle(element).fontFamily)).toContain('IBM Plex Mono')
  expect(await page.evaluate(() => getComputedStyle(document.body).fontFamily)).toContain('Charter')

  await page.reload()
  await waitForUiReady(page)
  await expect.poll(() => page.locator('[data-compose-editor]').evaluate(element => getComputedStyle(element).fontFamily)).toContain('IBM Plex Mono')
  expect(await page.evaluate(() => getComputedStyle(document.body).fontFamily)).toContain('Charter')

  await page.goto('/settings')
  await waitForUiReady(page)
  await expect(page.locator('[data-interface-font]')).toHaveValue('serif')
  await expect(page.locator('[data-composer-font]')).toHaveValue('monospace')
  expect(errors).toEqual([])
})
