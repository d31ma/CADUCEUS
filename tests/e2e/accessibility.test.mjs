import AxeBuilder from '@axe-core/playwright'
import { test, expect } from '@playwright/test'
import {
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

const AUTHENTICATED_ROUTES = [
  '/inbox',
  '/folder/drafts',
  '/folder/sent',
  '/folder/archive',
  '/folder/trash',
  '/compose',
  '/contacts',
  '/scheduled',
  '/reminders',
  '/calendar',
  '/team',
  '/activity',
  '/settings',
]

const WCAG_TAGS = ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa']

function violationSummary(route, violations) {
  return violations.flatMap(violation => violation.nodes.map(node => ({
    route,
    rule: violation.id,
    impact: violation.impact,
    target: node.target.join(' '),
    message: node.failureSummary,
  })))
}

async function authenticate(page) {
  const domain = `a11y-${Date.now()}-${Math.random().toString(36).slice(2)}.test`
  const email = `owner@${domain}`
  const messageId = `accessibility-${Date.now()}-${Math.random().toString(36).slice(2)}`
  await seedUser(email, { domains: [domain] })
  const session = await getToken(email)
  await seedDomain(session.token, domain)
  await useTestApi(page)
  await loginAs(page, session.token, session.email, session.role, session.domains)
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'accessibility@example.net',
    subject: 'Accessible message fixture',
    body: 'A stable message used by the WCAG route matrix.',
    messageId,
  })
  return { email, session, delivered: { ...delivered, emailId: delivered.emailId || messageId } }
}

async function scan(page, route) {
  await page.goto(route)
  await waitForUiReady(page)
  await expect(page.locator('.inbox-empty').filter({ hasText: /Loading/ })).toHaveCount(0)
  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze()
  return violationSummary(route, results.violations)
}

for (const viewport of [
  { name: 'phone', width: 320, height: 568 },
  { name: 'desktop', width: 1440, height: 900 },
]) {
  test(`all authenticated routes satisfy automated WCAG 2.2 AA checks at ${viewport.name} size`, async ({ page }) => {
    test.setTimeout(120_000)
    const errors = captureUiErrors(page)
    await page.setViewportSize(viewport)
    await authenticate(page)

    const violations = []
    for (const route of AUTHENTICATED_ROUTES) violations.push(...await scan(page, route))

    expect(violations).toEqual([])
    expect(errors).toEqual([])
  })
}

test('login and a populated message reader satisfy automated WCAG 2.2 AA checks', async ({ page }) => {
  test.setTimeout(60_000)
  await useTestApi(page)
  await page.goto('/')
  await waitForUiReady(page, { shell: false })
  const login = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze()
  expect(violationSummary('/', login.violations)).toEqual([])

  const { delivered } = await authenticate(page)
  await page.goto(`/email/${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)
  const reader = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze()
  expect(violationSummary(`/email/${delivered.emailId}`, reader.violations)).toEqual([])
})

test('screen-reader role navigation exposes the mailbox and reader flow', async ({ page }) => {
  const { delivered } = await authenticate(page)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.getByRole('main')).toHaveCount(1)
  await expect(page.getByRole('combobox', { name: 'Search mail' })).toBeVisible()
  await expect(page.getByRole('separator', { name: 'Resize sidebar' })).toHaveAttribute('aria-valuenow', /\d+/)

  await page.goto(`/email/${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)
  await expect(page.getByRole('heading', { name: 'Accessible message fixture' })).toBeVisible()
  const readerToolbar = page.locator('.email-toolbar')
  await expect(readerToolbar.getByRole('button', { name: 'Reply', exact: true })).toBeVisible()
  await expect(readerToolbar.getByRole('button', { name: 'Find in conversation' })).toBeVisible()
})

test('modal focus is trapped and restored for keyboard and contact dialogs', async ({ page }) => {
  const { email } = await authenticate(page)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.locator('.inbox-empty').filter({ hasText: /Loading/ })).toHaveCount(0)

  await page.waitForFunction(() => window._caduceusKeyboard?.getCatalog?.().length > 0)
  const returnTarget = page.getByRole('link', { name: 'CADUCEUS' })
  await returnTarget.focus()
  await page.keyboard.press('?')
  const shortcuts = page.getByRole('dialog', { name: 'Keyboard shortcuts' })
  await expect(shortcuts).toBeVisible()
  await expect(shortcuts).toContainText('Calendar')
  await page.keyboard.press('Shift+Tab')
  await expect(shortcuts.locator(':focus')).toHaveCount(1)
  await page.keyboard.press('Escape')
  await expect(returnTarget).toBeFocused()

  await page.goto('/contacts')
  await waitForUiReady(page)
  const createGroup = page.getByRole('button', { name: 'Create contact group' })
  await createGroup.click()
  const groupDialog = page.getByRole('dialog', { name: 'New group' })
  await expect(groupDialog).toBeVisible()
  await expect(groupDialog.getByRole('textbox', { name: 'Group name' })).toBeFocused()
  await page.keyboard.press('Shift+Tab')
  await expect(groupDialog.locator(':focus')).toHaveCount(1)
  await page.keyboard.press('Escape')
  await expect(createGroup).toBeFocused()
  expect(email).toContain('@')
})

test('forced colors, WCAG text spacing, and 400-percent-equivalent reflow preserve access', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 800 })
  await page.emulateMedia({ forcedColors: 'active', reducedMotion: 'reduce' })
  await authenticate(page)
  await page.goto('/inbox')
  await waitForUiReady(page)

  await page.addStyleTag({ content: `
    * { line-height: 1.5 !important; letter-spacing: 0.12em !important; word-spacing: 0.16em !important; }
    p { margin-bottom: 2em !important; }
  ` })
  const search = page.locator('[data-thread-search-input]')
  await search.focus()
  const focusStyle = await search.evaluate(element => getComputedStyle(element).outlineStyle)
  expect(focusStyle).not.toBe('none')

  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)
  expect(overflow).toBeLessThanOrEqual(1)

  const results = await new AxeBuilder({ page }).withTags(WCAG_TAGS).analyze()
  expect(violationSummary('/inbox?forced-colors', results.violations)).toEqual([])
})
