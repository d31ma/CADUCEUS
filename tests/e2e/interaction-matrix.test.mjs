/** Deterministic cross-route interaction and DuVay integrity coverage. */
import { test, expect } from '@playwright/test'
import {
  captureUiErrors,
  deliverEmail,
  expectDuVayIntegrity,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  uiFingerprint,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

async function authenticate(page) {
  const nonce = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `interaction-${nonce}.test`
  const email = `admin-${nonce}@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  return { email, token: account.token, domain }
}

async function assertDocumentFits(page) {
  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
  }))
  expect(dimensions.document - dimensions.viewport).toBeLessThanOrEqual(1)
}

async function waitForStableFingerprint(page, selector) {
  let previous = []
  let stableSamples = 0
  for (let attempt = 0; attempt < 30; attempt += 1) {
    const current = await uiFingerprint(page, selector)
    if (JSON.stringify(current) === JSON.stringify(previous)) stableSamples += 1
    else stableSamples = 0
    if (stableSamples >= 5) return current
    previous = current
    await page.waitForTimeout(150)
  }
  throw new Error(`${selector} did not settle`)
}

const ROUTES = [
  '/inbox',
  '/folder/drafts',
  '/folder/sent',
  '/folder/archive',
  '/folder/spam',
  '/folder/trash',
  '/scheduled',
  '/compose',
  '/contacts',
  '/calendar',
  '/team',
  '/settings',
]

test('complete route matrix remains stable in an RTL locale', async ({ page }) => {
  const errors = captureUiErrors(page)
  await page.setViewportSize({ width: 320, height: 568 })
  await authenticate(page)
  await page.goto('/settings')
  await waitForUiReady(page)

  const saved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.interfaceLocale === 'he')
  await page.locator('select[data-interface-locale]').selectOption('he')
  expect((await saved).status()).toBe(200)

  for (const viewport of [{ width: 320, height: 568 }, { width: 1440, height: 900 }]) {
    await page.setViewportSize(viewport)
    for (const route of ROUTES) {
      await page.goto(route)
      await waitForUiReady(page)
      await expect(page.locator('html')).toHaveAttribute('lang', 'he')
      await expect(page.locator('html')).toHaveAttribute('dir', 'rtl')
      await expect(page.locator('html')).toHaveAttribute('data-interface-catalog-ready', '')
      await expectDuVayIntegrity(page, expect)
      await assertDocumentFits(page)
      if (viewport.width === 1440 && route === '/inbox') {
        const sidebar = await page.locator('.app-sidebar').boundingBox()
        if (!sidebar) throw new Error('RTL sidebar is not visible on desktop')
        expect(Math.abs(sidebar.x + sidebar.width - viewport.width)).toBeLessThanOrEqual(1)
        const widthBefore = Number.parseFloat(await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--app-sidebar-width')))
        await page.locator('[data-column-resizer="sidebar"]').press('ArrowLeft')
        const widthAfter = Number.parseFloat(await page.evaluate(() =>
          getComputedStyle(document.documentElement).getPropertyValue('--app-sidebar-width')))
        expect(widthAfter).toBeGreaterThan(widthBefore)
      }
      const first = await waitForStableFingerprint(page, '.app-main')
      expect(await uiFingerprint(page, '.app-main')).toEqual(first)
    }
  }

  expect(errors).toEqual([])
})

for (const viewport of [
  { name: 'phone', width: 320, height: 568 },
  { name: 'tablet', width: 834, height: 1112 },
  { name: 'desktop', width: 1440, height: 900 },
]) {
  test(`all routes remain upgraded and contained at ${viewport.name}`, async ({ page }) => {
    const errors = captureUiErrors(page)
    await page.setViewportSize(viewport)
    await authenticate(page)

    for (const route of ROUTES) {
      await page.goto(route)
      await waitForUiReady(page)
      await expectDuVayIntegrity(page, expect)
      await assertDocumentFits(page)

      const first = await waitForStableFingerprint(page, '.app-main')
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
      expect(await uiFingerprint(page, '.app-main'), `${route} changed after settling`).toEqual(first)
    }

    expect(errors).toEqual([])
  })
}

test('shell interactions preserve the message column geometry', async ({ page }) => {
  const errors = captureUiErrors(page)
  await page.setViewportSize({ width: 1440, height: 900 })
  await authenticate(page)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await waitForStableFingerprint(page, '.app-main')

  const messageBox = () => page.locator('.mail-list-panel').boundingBox()
  const before = await messageBox()

  await page.getByRole('button', { name: 'Favorites' }).click()
  await expect(page.getByRole('button', { name: 'Favorites' })).not.toHaveAttribute('aria-expanded', 'true')
  const afterSection = await messageBox()
  expect(afterSection).toEqual(before)

  await page.getByRole('button', { name: 'Favorites' }).click()
  await page.getByRole('button', { name: 'Toggle inboxes' }).click()
  expect(await messageBox()).toEqual(before)

  const themeBefore = await page.locator('html').getAttribute('data-theme')
  await page.getByRole('button', { name: /Switch to (light|dark) theme/ }).click()
  await expect(page.locator('html')).not.toHaveAttribute('data-theme', themeBefore)

  await page.locator('.app-user-trigger').click()
  await expect(page.getByRole('menuitem', { name: 'Settings' })).toBeVisible()
  await page.keyboard.press('Escape')
  await page.locator('.app-user-trigger').click()
  await page.locator('.app-main').click({ position: { x: 10, y: 10 } })
  await expect(page.getByRole('menuitem', { name: 'Settings' })).toBeHidden()

  const sidebarWidth = await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--app-sidebar-width'))
  await page.getByRole('separator', { name: 'Resize sidebar' }).press('ArrowRight')
  expect(await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--app-sidebar-width'))).not.toBe(sidebarWidth)

  expect(errors).toEqual([])
})

test('phone shell exposes navigation without a bottom bar', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await authenticate(page)
  await page.goto('/inbox')
  await waitForUiReady(page)

  await expect(page.locator('.mobile-bottom-nav,.mobile-nav')).toHaveCount(0)
  await page.getByRole('button', { name: 'Open navigation menu' }).click()
  await expect(page.locator('.app-sidebar')).toBeVisible()
  await page.getByRole('link', { name: 'Calendar' }).click()
  await expect(page).toHaveURL(/\/calendar\/?$/)
  await expect(page.getByRole('button', { name: 'Open navigation menu' })).toBeVisible()
})

test('inbox controls update data without changing unrelated components', async ({ page }) => {
  const errors = captureUiErrors(page)
  const account = await authenticate(page)
  await seedDomain(account.token, account.domain)
  await deliverEmail({ recipient: account.email, sender: 'billing@vendor.test', subject: 'Blue invoice', body: 'blue searchable body' })
  await deliverEmail({ recipient: account.email, sender: 'alerts@vendor.test', subject: 'Green alert', body: 'green body' })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.locator('.email-row')).toHaveCount(2)

  await expect.poll(async () => (await uiFingerprint(page, '.app-topbar')).length).toBeGreaterThan(0)
  const themeControl = await uiFingerprint(page, '.app-topbar')
  const densityBefore = await page.locator('.inbox-list').getAttribute('data-density')
  await page.getByRole('button', { name: 'Change density' }).click()
  await expect(page.locator('.inbox-list')).not.toHaveAttribute('data-density', densityBefore)
  expect(await uiFingerprint(page, '.app-topbar')).toEqual(themeControl)

  await page.locator('[data-thread-search-input]').fill('blue searchable')
  await page.getByRole('button', { name: 'Search', exact: true }).click()
  await expect(page.getByText('Blue invoice')).toBeVisible()
  await expect(page.getByText('Green alert')).toBeHidden()
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  await expect(page.locator('.email-row')).toHaveCount(2)
  await expect(page.locator('[data-thread-search-input]')).toHaveValue('')

  await page.locator('.email-row').first().click({ position: { x: 80, y: 24 } })
  await expect(page.locator('.email-row').first()).toHaveClass(/selected/)
  await expect(page.locator('.email-body')).toBeVisible()
  await expect(page.locator('.email-back-btn')).toBeHidden()
  await expectDuVayIntegrity(page, expect)
  expect(errors).toEqual([])
})

test('compose DuVay fields, validation, attachment, and discard stay coherent', async ({ page }) => {
  const errors = captureUiErrors(page)
  await authenticate(page)
  await page.goto('/compose')
  await waitForUiReady(page)

  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect(page.getByText('To and Subject are required.')).toBeVisible()

  await page.locator('[data-compose-to] input').fill('recipient@example.com')
  await page.locator('[data-compose-subject] input').fill('Interaction matrix')
  await page.locator('[data-compose-editor]').fill('A stable message body')
  await page.locator('[data-attachment-input]').setInputFiles({
    name: 'matrix.txt',
    mimeType: 'text/plain',
    buffer: Buffer.from('attachment payload'),
  })
  await expect(page.getByText('matrix.txt')).toBeVisible()
  await page.getByRole('button', { name: 'Remove matrix.txt' }).click()
  await expect(page.getByText('matrix.txt')).toBeHidden()

  await page.getByRole('button', { name: 'Discard' }).click()
  await expect(page).toHaveURL(/\/inbox\/?$/)
  expect(errors).toEqual([])
})

test('settings reveals each authentication setup flow without component loss', async ({ page }) => {
  const errors = captureUiErrors(page)
  await authenticate(page)
  await page.goto('/settings')
  await waitForUiReady(page)

  await page.getByRole('button', { name: 'Use Purple' }).click()
  await expect(page.getByText('Current: Purple')).toBeVisible()

  await page.getByRole('button', { name: '+ Add passkey' }).click()
  await expect(page.locator('[data-settings-stepup]')).toBeVisible()
  await expect(page.getByText('Confirm it is you: enter your password and one recovery code.')).toBeVisible()

  await page.getByRole('button', { name: '+ Add authenticator app' }).click()
  await expect(page.locator('[data-settings-stepup]')).toBeVisible()
  await expect(page.getByText('Confirm it is you: enter your password and one recovery code.')).toBeVisible()

  await page.getByRole('button', { name: '+ Add rule' }).click()
  await expect(page.locator('input[placeholder="e.g. Move newsletters"]')).toBeVisible()
  await page.getByRole('button', { name: 'Cancel', exact: true }).click()

  await expectDuVayIntegrity(page, expect)
  expect(errors).toEqual([])
})
