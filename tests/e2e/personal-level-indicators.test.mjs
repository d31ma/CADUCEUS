import { expect, test } from '@playwright/test'
import {
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  uniqueEmail,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('shows direct and group indicators but not mailing-list delivery', async ({ page }) => {
  const email = uniqueEmail()
  const account = await seedUser(email)
  await seedDomain((await getToken(email)).token)

  const stamp = Date.now()
  const raw = (to, subject) => [
    `From: Sender <sender@outside.test>`,
    `To: ${to}`,
    `Message-ID: <${subject.replaceAll(' ', '-')}@outside.test>`,
    'Content-Type: text/plain; charset=utf-8',
    '',
    subject,
  ].join('\r\n')

  await deliverEmail({
    recipient: email,
    sender: 'sender@outside.test',
    subject: `Only you ${stamp}`,
    body: raw(email, `Only you ${stamp}`),
    messageId: `personal-only-${stamp}`,
  })
  await deliverEmail({
    recipient: email,
    sender: 'sender@outside.test',
    subject: `Group ${stamp}`,
    body: raw(`${email}, teammate@example.net`, `Group ${stamp}`),
    messageId: `personal-group-${stamp}`,
  })
  await deliverEmail({
    recipient: email,
    sender: 'sender@outside.test',
    subject: `List ${stamp}`,
    body: raw('staff@example.net', `List ${stamp}`),
    messageId: `personal-list-${stamp}`,
  })

  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.locator('.filter-strip w-chip').filter({ hasText: 'Threaded' }).click()

  const only = page.locator('.thread-summary', { hasText: `Only you ${stamp}` })
  const group = page.locator('.thread-summary', { hasText: `Group ${stamp}` })
  const list = page.locator('.thread-summary', { hasText: `List ${stamp}` })

  await expect(only.locator('[data-personal-level="2"]')).toHaveAttribute('aria-label', 'Sent only to you')
  await expect(group.locator('[data-personal-level="1"]')).toHaveAttribute('aria-label', 'Sent to you and others')
  await expect(list.locator('[data-personal-level]')).toHaveCount(0)
})
