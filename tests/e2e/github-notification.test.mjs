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

test('opens exact GitHub notification targets by toolbar and Mailspring shortcut only', async ({ page, context }) => {
  const errors = captureUiErrors(page)
  const domain = `github-action-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  const target = 'https://github.com/acme/widget/issues/42#issuecomment-7'
  const safeSubject = `GitHub notification ${Date.now()}`
  const lookalikeSubject = `GitHub sender lookalike ${Date.now()}`
  const hostileTargetSubject = `GitHub target lookalike ${Date.now()}`
  await deliverEmail({
    recipient: email,
    sender: 'notifications@github.com',
    subject: safeSubject,
    htmlBody: `<p>Issue update</p><a href="${target}"><span>View it on GitHub</span></a>`,
  })
  await deliverEmail({
    recipient: email,
    sender: 'notifications@github.com.evil.test',
    subject: lookalikeSubject,
    htmlBody: `<a href="${target}">View it on GitHub</a>`,
  })
  await deliverEmail({
    recipient: email,
    sender: 'notifications@github.com',
    subject: hostileTargetSubject,
    htmlBody: '<a href="https://github.com.evil.test/acme/widget/issues/42">View it on GitHub</a>',
  })
  await context.route('https://github.com/**', route => route.fulfill({
    status: 200,
    contentType: 'text/html',
    body: '<title>GitHub target</title><p>Exact GitHub target</p>',
  }))
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText(safeSubject, { exact: true }).first().click()
  const action = page.getByRole('button', { name: 'Visit thread on GitHub' })
  await expect(action).toBeVisible()

  await page.getByRole('button', { name: 'Star', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Unstar', exact: true })).toBeVisible()
  await expect(action).toBeVisible()

  const clickPopup = page.waitForEvent('popup')
  await action.click()
  const firstTarget = await clickPopup
  await expect(firstTarget).toHaveURL(target)
  await firstTarget.close()

  const shortcutPopup = page.waitForEvent('popup')
  await page.keyboard.press(process.platform === 'darwin' ? 'Meta+g' : 'Control+g')
  const secondTarget = await shortcutPopup
  await expect(secondTarget).toHaveURL(target)
  await secondTarget.close()

  await page.getByText(lookalikeSubject, { exact: true }).first().click()
  await expect(action).toHaveCount(0)
  await page.getByText(hostileTargetSubject, { exact: true }).first().click()
  await expect(action).toHaveCount(0)
  expect(errors).toEqual([])
})
