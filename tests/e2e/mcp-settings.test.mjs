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

test('scopes, persists, audits, rotates, and revokes MCP access responsively', async ({ page }) => {
  const domain = `mcp-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/settings')
  await waitForUiReady(page)

  const panel = page.locator('[data-mcp-settings]')
  const enabled = panel.getByRole('switch', { name: /Allow MCP connections/ })
  const spam = panel.getByRole('checkbox', { name: `${domain} · spam (spam)`, exact: true })
  const readWrite = panel.getByRole('radio', { name: 'Read and write', exact: true })
  await expect(enabled).toBeEnabled()
  await expect(enabled).not.toBeChecked()
  await expect(panel.getByText('No active token', { exact: true })).toBeVisible()

  await enabled.click()
  await readWrite.check()
  await spam.click()
  await expect(enabled).toBeChecked()
  await expect(readWrite).toBeChecked()
  await expect(spam).toBeChecked()

  const saved = page.waitForResponse(response =>
    response.url().endsWith('/mcp/config') && response.request().method() === 'PUT')
  await panel.getByRole('button', { name: 'Save MCP access', exact: true }).click()
  const savedResponse = await saved
  expect(savedResponse.status()).toBe(200)
  expect(savedResponse.request().postDataJSON()).toMatchObject({
    enabled: true,
    accessLevel: 'read-write',
    allowedDomains: [domain],
    excludedFolders: { [domain]: ['spam'] },
  })

  const rotated = page.waitForResponse(response =>
    response.url().endsWith('/mcp/token') && response.request().method() === 'POST')
  await panel.getByRole('button', { name: 'Generate token', exact: true }).click()
  expect((await rotated).status()).toBe(200)
  await expect(panel.getByText('Copy this token now', { exact: true })).toBeVisible()
  const capability = (await panel.locator('.mcp-secret').textContent())?.trim() || ''
  expect(capability).toMatch(/^mcp1\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]{43}$/)

  const toolResponse = await fetch(`${API}/mcp`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${capability}`,
      'Content-Type': 'application/json',
      Accept: 'application/json, text/event-stream',
    },
    body: JSON.stringify({
      jsonrpc: '2.0', id: 1, method: 'tools/call',
      params: { name: 'list_accounts', arguments: {} },
    }),
  })
  expect(toolResponse.status).toBe(200)
  expect(await toolResponse.text()).toContain(domain)

  await page.reload()
  await waitForUiReady(page)
  await expect(enabled).toBeEnabled()
  await expect(enabled).toBeChecked()
  await expect(readWrite).toBeChecked()
  await expect(spam).toBeChecked()
  await expect(panel.getByText('An active token exists', { exact: false })).toBeVisible()
  await expect(panel.locator('.mcp-secret')).toHaveCount(0)
  await expect(panel.getByText('list_accounts', { exact: true })).toBeVisible()
  await expect(panel.getByText('ok', { exact: true })).toBeVisible()

  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
    panel: document.querySelector('.settings-panel')?.scrollWidth || Infinity,
    panelClient: document.querySelector('.settings-panel')?.clientWidth || 0,
  }))
  expect(dimensions.document - dimensions.viewport).toBeLessThanOrEqual(1)
  expect(dimensions.panel - dimensions.panelClient).toBeLessThanOrEqual(1)

  const revoked = page.waitForResponse(response =>
    response.url().endsWith('/mcp/token') && response.request().method() === 'DELETE')
  await panel.getByRole('button', { name: 'Revoke', exact: true }).click()
  expect((await revoked).status()).toBe(200)
  await expect(panel.getByText('No active token', { exact: true })).toBeVisible()
  expect(errors).toEqual([])
})
