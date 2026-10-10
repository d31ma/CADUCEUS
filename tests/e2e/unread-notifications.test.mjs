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

test('persists per-account privacy controls and executes an authenticated notification action', async ({ page }) => {
  test.setTimeout(60_000)
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const primaryDomain = `notify-primary-${suffix}.test`
  const secondaryDomain = `notify-secondary-${suffix}.test`
  const email = `owner@${primaryDomain}`
  await seedUser(email, { domains: [primaryDomain, secondaryDomain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, primaryDomain)
  const delivered = await deliverEmail({
    recipient: email,
    sender: 'alerts@outside.test',
    subject: `Private notification ${suffix}`,
    body: 'This body must only be visible after authentication.',
    messageId: `notification-${suffix}`,
  })

  await page.setViewportSize({ width: 390, height: 844 })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, [primaryDomain, secondaryDomain])
  const errors = captureUiErrors(page)
  await page.goto('/settings')
  await waitForUiReady(page)

  const secondary = page.getByRole('checkbox', { name: secondaryDomain, exact: true })
  await expect(secondary).toBeChecked()
  const accountSaved = page.waitForResponse(response => {
    if (!response.url().endsWith('/users/me/preferences') || response.request().method() !== 'PUT') return false
    const domains = response.request().postDataJSON()?.newMailNotificationDomains
    return Array.isArray(domains) && domains.length === 1 && domains[0] === primaryDomain
  })
  await secondary.uncheck()
  expect((await accountSaved).status()).toBe(200)

  const privacy = page.locator('#new-mail-notification-privacy')
  const privacySaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences')
      && response.request().method() === 'PUT'
      && response.request().postDataJSON()?.newMailNotificationPrivacy === 'private')
  await privacy.selectOption('private')
  expect((await privacySaved).status()).toBe(200)

  const actions = page.getByRole('switch', { name: 'Show Mark read and Archive actions' })
  const actionsSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences')
      && response.request().method() === 'PUT'
      && response.request().postDataJSON()?.newMailNotificationActions === false)
  await actions.uncheck()
  expect((await actionsSaved).status()).toBe(200)

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted).toMatchObject({
    newMailNotificationDomains: [primaryDomain],
    newMailNotificationPrivacy: 'private',
    newMailNotificationActions: false,
  })

  const actionApplied = page.waitForResponse(response =>
    response.url().endsWith('/inbox/actions') && response.request().method() === 'POST')
  await page.goto(`/inbox?notificationAction=mark-read&notificationEmail=${encodeURIComponent(delivered.emailId)}`)
  await waitForUiReady(page)
  expect((await actionApplied).status()).toBe(200)
  await expect(page).toHaveURL(/\/inbox\/?$/)

  const stored = await fetch(`${API}/inbox/${encodeURIComponent(delivered.emailId)}`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(stored.read).toBe(true)
  await expect.poll(async () => (await page.locator('.app-main').boundingBox())?.width ?? Infinity)
    .toBeLessThanOrEqual(390)
  expect(errors).toEqual([])
})
