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

test('collapses recognized HTML and plain-text quotations without discarding them', async ({ page }) => {
  const errors = captureUiErrors(page)
  const domain = `quoted-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  const htmlSubject = `HTML quotation ${Date.now()}`
  const textSubject = `Plain quotation ${Date.now()}`
  const htmlId = `quoted-html-${Date.now()}`
  const textId = `quoted-text-${Date.now()}`

  await deliverEmail({
    recipient: email,
    sender: 'reply@example.test',
    subject: htmlSubject,
    messageId: htmlId,
    htmlBody: '<p>Fresh HTML reply</p><p data-caduceus-quoted>Important visible line</p><blockquote><p>Earlier HTML reply</p><blockquote>Oldest</blockquote></blockquote>',
  })
  await deliverEmail({
    recipient: email,
    sender: 'reply@example.test',
    subject: textSubject,
    messageId: textId,
    body: 'Fresh plain reply\n\nOn Fri, Aug 14, Alice wrote:\n> Earlier plain reply',
  })

  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)

  await page.locator(`[data-email-id="${htmlId}"] .email-avatar`).click()
  const htmlFrame = page.frameLocator('iframe[title="Email HTML body"]')
  await expect(htmlFrame.getByText('Fresh HTML reply', { exact: true })).toBeVisible()
  await expect(htmlFrame.getByText('Important visible line', { exact: true })).toBeVisible()
  await expect(htmlFrame.getByText('Earlier HTML reply', { exact: true })).toBeHidden()
  const toggle = page.getByRole('button', { name: 'Show quoted content' })
  await expect(toggle).toHaveAttribute('aria-expanded', 'false')
  await toggle.click()
  await expect(htmlFrame.getByText('Earlier HTML reply', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Hide quoted content' })).toHaveAttribute('aria-expanded', 'true')

  await page.locator(`[data-email-id="${textId}"] .email-avatar`).click()
  await expect(page.locator('.email-body')).toHaveText('Fresh plain reply')
  await expect(page.locator('.email-quoted-plain')).toHaveCount(0)
  await page.getByRole('button', { name: 'Show quoted content' }).click()
  await expect(page.locator('.email-quoted-plain')).toContainText('Earlier plain reply')
  expect(errors).toEqual([])
})
