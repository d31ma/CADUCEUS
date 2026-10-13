import { createHmac } from 'node:crypto'
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

const SECRET = 'caduceus-e2e-inbound-secret'

test('schedules, lists, dismisses, and independently resurfaces follow-up reminders', async ({ page }) => {
  const domain = `reminders-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  const subject = `Browser follow-up ${Date.now()}`
  await page.goto('/compose')
  await waitForUiReady(page)
  await page.locator('[data-compose-to] input').fill('recipient@outside.test')
  await page.locator('[data-compose-subject] input').fill(subject)
  await page.locator('[data-compose-editor]').fill('Please reply when you can.')

  const chooserButton = page.getByRole('button', { name: 'Set a follow-up reminder if nobody replies' })
  await chooserButton.click()
  const chooser = page.getByRole('dialog', { name: 'Remind me if nobody replies' })
  await expect(chooser).toBeVisible()
  await expect(chooser.getByRole('button', { name: 'Tomorrow morning' })).toBeVisible()
  await expect(chooser.getByRole('button', { name: 'In 3 days' })).toBeVisible()
  await expect(chooser.getByRole('button', { name: 'Next week' })).toBeVisible()
  await expect(chooser.locator('input[type="datetime-local"]')).toBeVisible()
  await chooser.getByRole('button', { name: 'In 3 days' }).click()
  await expect(chooser).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Remove follow-up reminder' })).toBeVisible()
  await expect(page.locator('[data-compose-editor]')).toContainText('Please reply when you can.')

  const sendResponsePromise = page.waitForResponse(response =>
    response.url().endsWith('/send') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  const sendResponse = await sendResponsePromise
  expect(sendResponse.status()).toBe(200)
  expect(sendResponse.request().postDataJSON().remindAt).toBeTruthy()
  await page.waitForURL(/\/inbox\/?$/)

  await page.getByRole('link', { name: 'Reminders', exact: true }).click()
  await expect(page).toHaveURL(/\/reminders\/?$/)
  await expect(page.getByText(subject, { exact: true })).toBeVisible()
  await expect(page.getByText(/Waiting until/)).toBeVisible()
  await page.getByRole('button', { name: 'Dismiss', exact: true }).click()
  await expect(page.getByText(subject, { exact: true })).toHaveCount(0)

  const dueSubject = `Worker due ${Date.now()}`
  const remindAt = new Date(Date.now() + 1_000).toISOString()
  const send = await fetch(`${API}/send`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${account.token}` },
    body: JSON.stringify({ to: ['recipient@outside.test'], subject: dueSubject, text: 'No reply yet.', remindAt }),
  })
  expect(send.ok).toBe(true)
  await new Promise(resolve => setTimeout(resolve, 1_100))

  const body = '{}'
  const tick = await fetch(`${API}/reminders/tick`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Caduceus-Signature': createHmac('sha256', SECRET).update(body).digest('hex'),
    },
    body,
  })
  expect(tick.ok).toBe(true)
  expect(await tick.json()).toMatchObject({ due: 1, surfaced: 1, failed: 0 })

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText(`Re: ${dueSubject}`, { exact: true }).first().click()
  await expect(page.getByText('Follow-up reminder: nobody has replied', { exact: false }).first()).toBeVisible()
  expect(errors).toEqual([])
})
