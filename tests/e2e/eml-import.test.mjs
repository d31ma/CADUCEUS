import { test, expect } from '@playwright/test'
import { getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function authenticatedMailbox(page) {
  const domain = `eml-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)
  return { domain, email }
}

test('imports multiple EML files with progress and exact mailbox placement', async ({ page }) => {
  const { email } = await authenticatedMailbox(page)
  const first = [
    'From: First Sender <first@sender.test>',
    `To: ${email}`,
    'Date: Fri, 14 Aug 2026 13:00:00 +0000',
    'Message-ID: <browser-import-one@sender.test>',
    'Subject: First imported EML',
    '',
    'First imported body',
  ].join('\r\n')
  const second = [
    'From: Second Sender <second@sender.test>',
    `To: ${email}`,
    'Date: Fri, 14 Aug 2026 13:01:00 +0000',
    'Message-ID: <browser-import-two@sender.test>',
    'Subject: Second imported EML',
    '',
    'Second imported body',
  ].join('\r\n')

  await page.locator('[data-eml-import]').setInputFiles([
    { name: 'first.eml', mimeType: 'message/rfc822', buffer: Buffer.from(first) },
    { name: 'second.eml', mimeType: 'message/rfc822', buffer: Buffer.from(second) },
  ])

  await expect(page.getByText('Imported 2 messages into Inbox.')).toBeVisible()
  await expect(page.getByText('First imported EML')).toBeVisible()
  await expect(page.getByText('Second imported EML')).toBeVisible()
})

test('cancels a stalled EML import without starting remaining files', async ({ page }) => {
  const { email } = await authenticatedMailbox(page)
  // The first upload never gets an answer; the browser cancels it when the
  // import is canceled.
  await page.route('**/inbox/import/eml', () => {})
  const source = Buffer.from(`From: sender@sender.test\r\nTo: ${email}\r\nSubject: Never committed\r\n\r\nBody`)
  await page.locator('[data-eml-import]').setInputFiles([
    { name: 'stalled.eml', mimeType: 'message/rfc822', buffer: source },
    { name: 'remaining.eml', mimeType: 'message/rfc822', buffer: source },
  ])

  await expect(page.locator('[data-eml-import-progress]')).toBeVisible()
  await page.locator('[data-eml-import-progress]').getByRole('button', { name: 'Cancel' }).click()
  await expect(page.locator('[data-eml-import-progress]')).toHaveCount(0)
  await expect(page.getByText('EML import canceled after 0 messages.')).toBeVisible()
  await expect(page.getByText('Never committed')).toHaveCount(0)
})
