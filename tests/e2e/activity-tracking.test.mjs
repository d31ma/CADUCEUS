import { test, expect } from '@playwright/test'
import {
  captureUiErrors,
  getToken,
  loginAs,
  seedTrackingEvent,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('opt-in delivery insights produce a private Activity feed and message history', async ({ page }) => {
  const domain = `activity-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/compose')
  await waitForUiReady(page)
  const openToggle = page.locator('[data-open-tracking]')
  const linkToggle = page.locator('[data-link-tracking]')
  await expect(openToggle).not.toBeChecked()
  await expect(linkToggle).not.toBeChecked()
  await expect(page.locator('#compose-tracking-disclosure')).toContainText('Off by default')

  await page.locator('[data-compose-to] input').fill('recipient@outside.test')
  await page.locator('[data-compose-subject] input').fill('Tracked quarterly proposal')
  await page.locator('[data-compose-editor]').evaluate(editor => {
    editor.innerHTML = '<p>Please review the <a href="https://example.org/proposal">proposal</a>.</p>'
    editor.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText' }))
  })
  await openToggle.check()
  await linkToggle.check()

  const sendResponsePromise = page.waitForResponse(response =>
    response.url().endsWith('/send') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  const sendResponse = await sendResponsePromise
  expect(sendResponse.status()).toBe(200)
  const request = sendResponse.request().postDataJSON()
  expect(request.openTracking).toBe(true)
  expect(request.linkTracking).toBe(true)
  expect(request.trackingId).toBeUndefined()
  expect(request.html).toContain('https://example.org/proposal')
  expect(request.html).not.toContain('/track/')
  const sent = await sendResponse.json()
  const messageId = sent.undoId || sent.messageId || sent.id
  expect(messageId).toBeTruthy()
  await page.waitForURL(/\/inbox\/?$/)

  await seedTrackingEvent(account.token, { messageId, type: 'open', recipient: 'recipient@outside.test' })
  await seedTrackingEvent(account.token, { messageId, type: 'click', recipient: 'recipient@outside.test', linkIndex: 0 })
  await page.evaluate(owner => localStorage.setItem(`caduceus_activity_viewed:${owner}`, '2020-01-01T00:00:00.000Z'), email)

  await page.goto('/inbox')
  await waitForUiReady(page)
  const activityLink = page.getByRole('link', { name: /Activity/ })
  await expect(activityLink.locator('.folder-count')).toHaveText('2')
  await activityLink.click()
  await expect(page).toHaveURL(/\/activity\/?$/)
  await expect(page.locator('.activity-page')).toBeVisible()
  await expect(page.locator('.activity-metric').filter({ hasText: 'Tracking use' })).toContainText('100%')
  await expect(page.locator('.activity-metric').filter({ hasText: 'Open rate' })).toContainText('100%')
  await expect(page.locator('.activity-metric').filter({ hasText: 'Click rate' })).toContainText('100%')
  await expect(page.locator('.activity-event-list .activity-event')).toHaveCount(2)
  await expect(page.getByText('Tracked quarterly proposal').first()).toBeVisible()
  await expect(activityLink.locator('.folder-count')).toHaveText('')

  await page.getByRole('tab', { name: 'Clicks' }).click()
  await expect(page.locator('.activity-event-list .activity-event')).toHaveCount(1)
  await expect(page.locator('.activity-event').first()).toContainText('Clicked')
  await page.locator('.activity-event').first().click()
  await expect(page).toHaveURL(new RegExp(`/email/${encodeURIComponent(messageId)}`))
  await expect(page.locator('[data-tracking-insights]')).toBeVisible()
  await expect(page.locator('[data-tracking-insights] summary')).toContainText('1 open · 1 click')
  await page.locator('[data-tracking-insights] summary').click()
  await expect(page.locator('.email-tracking-events li')).toHaveCount(2)

  expect(errors).toEqual([])
})
