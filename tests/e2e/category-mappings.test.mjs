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

test('maps an exact mailbox physical folder to Inbox across settings, reload, and mobile UI', async ({ page }) => {
  const domain = `categories-${Date.now()}.test`
  const mailbox = `owner@${domain}`
  const subject = `Mapped category ${Date.now()}`
  await seedUser(mailbox, { domains: [domain], role: 'admin' })
  const account = await getToken(mailbox)
  await seedDomain(account.token, domain)
  const delivered = await deliverEmail({ recipient: mailbox, subject, body: 'Physical priority folder' })

  const moved = await fetch(`${API}/inbox/${encodeURIComponent(delivered.emailId)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${account.token}` },
    body: JSON.stringify({ folder: 'priority' }),
  })
  expect(moved.status).toBe(200)

  await page.setViewportSize({ width: 390, height: 844 })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)
  await page.goto('/settings')
  await waitForUiReady(page)

  const mapper = page.locator('[data-category-mapper]')
  const inboxRole = mapper.getByLabel(`${domain} · Inbox`, { exact: true })
  await expect(inboxRole).toBeVisible()
  await expect(page.locator('#caduceus-category-paths option')).toHaveCount(7)
  await expect.poll(async () => (await mapper.boundingBox())?.width ?? Infinity).toBeLessThanOrEqual(390)

  const saved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT')
  await inboxRole.fill('priority')
  await mapper.getByRole('button', { name: 'Save folder roles' }).click()
  const savedResponse = await saved
  expect(savedResponse.request().postDataJSON()?.categoryMappings?.[domain]?.inbox).toBe('priority')
  expect(savedResponse.status()).toBe(200)

  await page.goto('/inbox')
  await waitForUiReady(page)
  await expect(page.getByText(subject, { exact: true }).first()).toBeVisible()
  const logicalInbox = await fetch(`${API}/inbox?folder=inbox`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(logicalInbox.find(message => message.id === delivered.emailId)).toMatchObject({
    folder: 'priority',
    systemFolder: 'inbox',
  })
  const preferences = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(preferences.categoryMappings?.[domain]?.inbox).toBe('priority')

  await page.goto('/settings')
  await waitForUiReady(page)
  await page.reload()
  await waitForUiReady(page)
  await expect(mapper.getByLabel(`${domain} · Inbox`, { exact: true })).toHaveValue('priority')
  expect(errors).toEqual([])
})
