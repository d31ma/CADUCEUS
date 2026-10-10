import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

async function authenticate(page, label) {
  const nonce = `${label}-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const domain = `${nonce}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  return { ...account, domain }
}

async function createContact(token, name, email) {
  const response = await fetch(`${API}/contacts`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ name, emails: [{ value: email, type: 'work' }] }),
  })
  if (!response.ok) throw new Error(`Contact seed failed: ${await response.text()}`)
}

test('offers caret-aware token and contact suggestions and searches from the keyboard', async ({ page }) => {
  const errors = captureUiErrors(page)
  const account = await authenticate(page, 'thread-search-keyboard')
  await createContact(account.token, 'Grace Hopper', 'grace@example.net')
  const key = `compiler-${Date.now()}`
  await deliverEmail({
    recipient: account.email,
    sender: 'grace@example.net',
    subject: `${key} Planning`,
    body: 'Bring the launch checklist.',
    messageId: `${key}-match`,
  })
  await deliverEmail({
    recipient: account.email,
    sender: 'ada@example.net',
    subject: `${key} Hidden`,
    body: 'A different message.',
    messageId: `${key}-hidden`,
  })

  await page.goto('/inbox')
  await waitForUiReady(page)
  const search = page.locator('[data-thread-search-input]')
  await search.focus()
  await expect(page.getByRole('listbox', { name: 'Search suggestions' })).toBeVisible()
  await search.fill('fr')
  await expect(search).toHaveValue('fr')
  await expect(page.getByRole('option', { name: /^from:/ })).toBeVisible()
  await search.press('Enter')
  await expect(search).toHaveValue('from: ')

  const contactSuggestions = page.waitForResponse(response => {
    const url = new URL(response.url())
    return url.pathname.endsWith('/contacts/suggestions') && url.searchParams.get('q') === 'grace'
  })
  await search.fill('from:grace')
  expect((await contactSuggestions).status()).toBe(200)
  const grace = page.getByRole('option', { name: /Grace Hopper.*grace@example\.net/ })
  await expect(grace).toBeVisible()
  await grace.click()
  await expect(search).toHaveValue('from:"grace@example.net" ')
  await expect(page.getByText(`${key} Planning`)).toBeVisible()
  await expect(page.getByText(`${key} Hidden`)).not.toBeVisible()
  await expect(page.locator('.search-token')).toContainText('from:')
  expect(errors).toEqual([])
})

test('runs boolean queries against threads and keeps suggestions inside a phone viewport', async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 568 })
  const errors = captureUiErrors(page)
  const account = await authenticate(page, 'thread-search-phone')
  const key = `threaded-${Date.now()}`
  await deliverEmail({
    recipient: account.email,
    sender: 'alerts@example.net',
    subject: `${key} Release`,
    body: 'Production checklist.',
    messageId: `${key}-thread`,
  })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText('Threaded', { exact: true }).click()
  const search = page.locator('[data-thread-search-input]')
  await search.fill(`(subject:"${key} Release" OR from:missing) NOT in:trash`)
  await expect(search).toHaveValue(`(subject:"${key} Release" OR from:missing) NOT in:trash`)
  const responsePromise = page.waitForResponse(response => (
    response.url().includes('/threads?') && new URL(response.url()).searchParams.has('q')
  ))
  await search.press('Enter')
  const response = await responsePromise
  expect(response.status()).toBe(200)
  await expect(page.locator('.thread-summary').filter({ hasText: `${key} Release` })).toBeVisible()

  await waitForUiReady(page)
  await search.focus()
  await search.fill('is:')
  await expect(page.getByRole('option', { name: /^is:unread/ })).toBeVisible()
  const dimensions = await page.evaluate(() => ({
    viewport: innerWidth,
    document: document.documentElement.scrollWidth,
    editor: document.querySelector('.search-editor')?.getBoundingClientRect().toJSON(),
    suggestions: document.querySelector('.search-suggestions')?.getBoundingClientRect().toJSON(),
  }))
  expect(dimensions.document - dimensions.viewport).toBeLessThanOrEqual(1)
  expect(dimensions.editor.x).toBeGreaterThanOrEqual(0)
  expect(dimensions.editor.x + dimensions.editor.width).toBeLessThanOrEqual(dimensions.viewport)
  expect(dimensions.suggestions.x).toBeGreaterThanOrEqual(0)
  expect(dimensions.suggestions.x + dimensions.suggestions.width).toBeLessThanOrEqual(dimensions.viewport)
  expect(errors).toEqual([])
})
