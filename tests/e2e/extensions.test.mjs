import { test, expect } from '@playwright/test'
import {
  captureUiErrors,
  getToken,
  loginAs,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

const packageBundle = {
  manifest: {
    schemaVersion: 1,
    id: 'example.command-center',
    displayName: 'Command Center',
    description: 'Adds one bounded application command.',
    version: '1.0.0',
    kind: 'plugin',
    engine: { caduceus: '26' },
    permissions: ['ui:notify'],
    contributions: [{ id: 'status', label: 'Extension status', slot: 'app-toolbar' }],
  },
  source: 'caduceus.activate({ onCommand(command, context) { return { notify: `${command.id}:${context.slot}` } } })',
}

const themeBundle = {
  manifest: {
    schemaVersion: 1,
    id: 'example.midnight',
    displayName: 'Midnight',
    description: 'A declarative local palette.',
    version: '1.0.0',
    kind: 'theme',
    engine: { caduceus: '26' },
    permissions: [],
    theme: { mode: 'dark', tokens: { primary: '#88aaff', surface: '#101218', text: '#f5f7ff' } },
  },
  source: '',
}

test('runs the complete trusted plugin and declarative theme lifecycle', async ({ page }) => {
  const domain = `extensions-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/settings#extensions')
  await waitForUiReady(page)
  const settings = page.locator('[data-extension-settings]')
  await expect(settings.getByText('No trusted extensions are installed for this mailbox.')).toBeVisible()

  const inspectedResponse = page.waitForResponse(response => response.url().endsWith('/extensions/inspect') && response.request().method() === 'POST')
  await settings.locator('[data-extension-package]').setInputFiles({
    name: 'command-center.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify(packageBundle)),
  })
  const inspected = await inspectedResponse
  expect({ status: inspected.status(), body: await inspected.json() }).toMatchObject({ status: 200 })
  const review = settings.locator('[data-extension-trust-review]')
  await expect(review.getByText('Command Center 1.0.0')).toBeVisible()
  await expect(review.getByText('Show a local notification')).toBeVisible()
  await expect(review.locator('.extension-digest')).toHaveText(/^[a-f0-9]{64}$/)
  await expect(page.getByRole('button', { name: 'Extension status' })).toHaveCount(0)

  await review.getByRole('button', { name: 'Trust exact digest and install' }).click()
  const packageRow = settings.locator('[data-extension-id="example.command-center"]')
  await expect(packageRow).toBeVisible()
  await expect(packageRow.getByRole('button', { name: 'Enable' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Extension status' })).toHaveCount(0)

  const enabledResponse = page.waitForResponse(response => response.url().includes('/extensions/') && response.request().method() === 'PUT')
  await packageRow.getByRole('button', { name: 'Enable' }).click()
  const enabled = await enabledResponse
  expect({ url: enabled.url(), status: enabled.status(), body: await enabled.json() }).toMatchObject({
    url: expect.stringMatching(/\/extensions\/example\.command-center$/),
    status: 200,
  })
  const command = page.getByRole('button', { name: 'Extension status' })
  await expect(command).toBeVisible()
  await expect(command).toBeEnabled()
  const sandbox = page.locator('iframe[title="Command Center extension sandbox"]')
  await expect(sandbox).toHaveAttribute('sandbox', 'allow-scripts')
  await expect(sandbox).toHaveAttribute('referrerpolicy', 'no-referrer')
  const sandboxDocument = await sandbox.getAttribute('srcdoc')
  expect(sandboxDocument).toContain("connect-src 'none'")
  expect(sandboxDocument).toContain('worker-src blob:')
  expect(sandboxDocument).not.toContain(packageBundle.source)

  await command.click()
  await expect(page.locator('.shell-toast-message')).toHaveText('status:app-toolbar')

  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
  }))
  expect(dimensions.document - dimensions.viewport).toBeLessThanOrEqual(1)

  const removedResponse = page.waitForResponse(response => response.url().endsWith('/extensions/example.command-center') && response.request().method() === 'DELETE')
  const removalDialog = page.waitForEvent('dialog').then(async dialog => {
    const question = dialog.message()
    await dialog.accept()
    expect(question).toBe('Remove “Command Center” and its trusted code?')
  })
  const [removed] = await Promise.all([
    removedResponse,
    removalDialog,
    packageRow.getByRole('button', { name: 'Remove' }).click(),
  ])
  expect({ status: removed.status(), body: await removed.json() }).toMatchObject({ status: 200, body: { deleted: true } })
  await expect(packageRow).toHaveCount(0)
  await expect(command).toHaveCount(0)
  await expect(sandbox).toHaveCount(0)

  await settings.locator('[data-extension-package]').setInputFiles({
    name: 'midnight.json',
    mimeType: 'application/json',
    buffer: Buffer.from(JSON.stringify(themeBundle)),
  })
  await expect(review.getByText('Midnight 1.0.0')).toBeVisible()
  await expect(review.getByText('Requests no permissions.')).toBeVisible()
  await review.getByRole('button', { name: 'Trust exact digest and install' }).click()
  const themeRow = settings.locator('[data-extension-id="example.midnight"]')
  await expect(themeRow).toBeVisible()
  await themeRow.getByRole('button', { name: 'Enable' }).click()
  // The Go runtime sets the theme's variables on <html> and marks it; the
  // JavaScript runtime's <style id="caduceus-extension-theme"> is gone.
  await expect(page.locator('html')).toHaveAttribute('data-extension-theme', 'enabled')
  await expect.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--ms-primary').trim())).toBe('#88aaff')
  await themeRow.getByRole('button', { name: 'Disable' }).click()
  await expect(page.locator('html')).not.toHaveAttribute('data-extension-theme', 'enabled')
  await expect.poll(() => page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--ms-primary').trim())).toBe('#1f6f8b')
  page.once('dialog', dialog => dialog.accept())
  await themeRow.getByRole('button', { name: 'Remove' }).click()
  await expect(themeRow).toHaveCount(0)
  expect(errors).toEqual([])
})
