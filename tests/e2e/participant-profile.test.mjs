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

test('shows tenant-local contact details and related mail in a durable responsive sidebar', async ({ page }) => {
  const domain = `profiles-${Date.now()}.test`
  const aliceEmail = `alice@${domain}`
  const bobEmail = `bob@${domain}`
  const participant = 'grace@outside.test'
  await seedUser(aliceEmail, { domains: [domain], role: 'admin' })
  await seedUser(bobEmail, { domains: [domain], role: 'viewer' })
  const alice = await getToken(aliceEmail)
  await seedDomain(alice.token, domain)

  const contactResponse = await fetch(`${API}/contacts`, {
    method: 'POST',
    headers: { Authorization: `Bearer ${alice.token}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({
      name: 'Grace Hopper',
      title: 'Rear Admiral',
      company: 'US Navy',
      emails: [{ value: participant, type: 'work' }],
      phones: [{ value: '+1 555 0100', type: 'work' }],
      notes: 'Compiler pioneer',
    }),
  })
  expect(contactResponse.status).toBe(200)

  for (let index = 1; index <= 5; index += 1) {
    await deliverEmail({
      recipient: aliceEmail,
      sender: participant,
      subject: `Local profile ${index}`,
      body: `Authorized related message ${index}`,
      messageId: `profile-alice-${index}-${Date.now()}`,
    })
  }
  await deliverEmail({
    recipient: bobEmail,
    sender: participant,
    subject: 'Bob tenant secret',
    body: 'This same-domain mailbox content must never appear for Alice.',
    messageId: `profile-bob-${Date.now()}`,
  })
  await expect.poll(async () => {
    const response = await fetch(`${API}/inbox`, { headers: { Authorization: `Bearer ${alice.token}` } })
    const emails = await response.json()
    return Array.isArray(emails) ? emails.filter(email => email.sender === participant).length : 0
  }, { timeout: 15_000 }).toBe(5)

  await page.setViewportSize({ width: 1920, height: 1080 })
  await useTestApi(page)
  await loginAs(page, alice.token, alice.email, alice.role, alice.domains)
  const errors = captureUiErrors(page)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText('Local profile 5', { exact: true }).click()
  await expect(page.locator('.email-body')).toBeVisible()

  await page.getByRole('button', { name: 'Show contact sidebar' }).click()
  const sidebar = page.getByRole('complementary', { name: 'Contact context' })
  await expect(sidebar).toBeVisible()
  await expect(sidebar.getByRole('heading', { name: 'Grace Hopper' })).toBeVisible()
  await expect(sidebar.getByText('Rear Admiral · US Navy')).toBeVisible()
  await expect(sidebar.getByText('Compiler pioneer')).toBeVisible()
  await expect(sidebar.getByText('Public profile lookup is off. CADUCEUS has not sent this address to GitHub.')).toBeVisible()
  await expect(sidebar.getByText('Bob tenant secret')).toHaveCount(0)
  await expect(sidebar.getByRole('button', { name: /Local profile/ })).toHaveCount(3)
  await sidebar.getByRole('button', { name: 'Show all' }).click()
  await expect(sidebar.getByRole('button', { name: /Local profile/ })).toHaveCount(5)

  await page.reload()
  await waitForUiReady(page)
  await page.getByText('Local profile 5', { exact: true }).click()
  await expect(page.locator('.email-body')).toBeVisible()
  await expect(page.getByRole('complementary', { name: 'Contact context' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Hide contact sidebar' }).first()).toBeVisible()

  await page.setViewportSize({ width: 390, height: 844 })
  const mobileSidebar = page.getByRole('complementary', { name: 'Contact context' })
  await expect(mobileSidebar).toBeVisible()
  await expect.poll(async () => (await mobileSidebar.boundingBox())?.width ?? Number.POSITIVE_INFINITY).toBeLessThanOrEqual(390)
  await mobileSidebar.getByRole('button', { name: 'Hide contact sidebar' }).click()
  await expect(mobileSidebar).toBeHidden()
  await expect(page.getByRole('button', { name: 'Show contact sidebar' })).toHaveAttribute('aria-pressed', 'false')
  expect(errors).toEqual([])
})
