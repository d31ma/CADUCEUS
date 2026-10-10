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

test('sends from a source conversation, archives it, and restores it on undo', async ({ page }) => {
  const domain = `send-archive-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await seedDomain(account.token, domain)
  const subject = `Archive browser ${Date.now()}`
  await deliverEmail({
    recipient: email,
    sender: 'sender@outside.test',
    subject,
    body: 'Archive this conversation after replying.',
    messageId: `archive-browser-${Date.now()}`,
  })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText(subject, { exact: true }).first().click()
  await page.getByRole('button', { name: 'Reply', exact: true }).first().click()
  await page.waitForURL(/\/compose\/?$/)
  await waitForUiReady(page)
  await expect(page.getByRole('button', { name: 'Send & Archive', exact: true })).toBeVisible()
  await expect(page.locator('[data-compose-to] input')).toHaveValue('sender@outside.test')
  await expect(page.locator('[data-compose-subject] input')).toHaveValue(`Re: ${subject}`)
  await page.locator('[data-compose-editor]').fill('Reply and archive.')

  const sendResponse = page.waitForResponse(response => response.url().endsWith('/send') && response.request().method() === 'POST')
  await page.getByRole('button', { name: 'Send & Archive', exact: true }).click()
  const sent = await sendResponse
  expect(sent.status(), sent.status() === 200 ? '' : await sent.text()).toBe(200)
  await page.waitForURL(/\/inbox\/?$/)
  await expect(page.getByText(subject, { exact: true })).toHaveCount(0)
  const archived = await fetch(`${API}/inbox?folder=archive`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
  expect(archived.some(message => message.subject === subject && message.folder === 'archive')).toBe(true)
  await expect(page.locator('#caduceus-toast-action')).toHaveText('Undo')

  const undoResponse = page.waitForResponse(response => response.url().endsWith('/send/undo') && response.request().method() === 'POST')
  await page.locator('#caduceus-toast-action').click()
  expect((await undoResponse).status()).toBe(200)
  await expect(page.getByText(subject, { exact: true }).first()).toBeVisible()

  const inbox = await fetch(`${API}/inbox`, { headers: { Authorization: `Bearer ${account.token}` } }).then(response => response.json())
  expect(inbox.some(message => message.subject === subject && message.folder === 'inbox')).toBe(true)
  expect(errors).toEqual([])
})
