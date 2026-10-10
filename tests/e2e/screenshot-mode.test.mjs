import { test, expect } from '@playwright/test'
import { deliverEmail, getToken, loginAs, seedDomain, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

test('masks PII across the app and message frames until explicitly disabled', async ({ page }) => {
  const domain = `privacy-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `private.owner@${domain}`
  const subject = 'Confidential acquisition — Project Juniper'
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  await deliverEmail({
    recipient: email,
    sender: 'ceo@private-vendor.test',
    subject,
    htmlBody: '<p>Revenue is <strong>$8,430,219</strong>.</p><img alt="Private logo" src="data:image/png;base64,iVBORw0KGgo=" />',
  })
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)
  const row = page.locator('.email-row').filter({ hasText: subject })
  await expect(row).toBeVisible()

  await page.evaluate(value => { document.title = value }, subject)
  await page.locator('.app-user-trigger').click()
  await page.getByRole('menuitem', { name: 'Screenshot mode' }).click()

  await expect(page.locator('html')).toHaveAttribute('data-screenshot-mode', 'true')
  await expect(page).toHaveTitle('CADUCEUS')
  await expect(page.getByRole('button', { name: 'Exit screenshot mode' })).toBeVisible()
  const subjectPrivacy = await row.locator('.email-subject-text').evaluate(element => {
    const style = getComputedStyle(element)
    return { color: style.color, textSecurity: style.webkitTextSecurity }
  })
  expect(
    subjectPrivacy.textSecurity === 'disc' || subjectPrivacy.color === 'rgba(0, 0, 0, 0)',
    `Expected masked subject styles, received ${JSON.stringify(subjectPrivacy)}`,
  ).toBe(true)
  expect(await page.locator('.app-layout').evaluate(element => getComputedStyle(element).filter)).toContain('blur')

  await row.getByRole('button', { name: /^Open message/ }).click({ position: { x: 120, y: 32 } })
  const frame = page.locator('.email-html-frame')
  await expect(frame).toBeVisible()
  await expect.poll(() => frame.evaluate(element => Boolean(
    /** @type {HTMLIFrameElement} */ (element).contentDocument?.getElementById('caduceus-screenshot-mode-styles'),
  ))).toBe(true)

  await page.getByRole('button', { name: 'Exit screenshot mode' }).click()
  await expect(page.locator('html')).not.toHaveAttribute('data-screenshot-mode', 'true')
  await expect(page).toHaveTitle(subject)
  await expect.poll(() => frame.evaluate(element => Boolean(
    /** @type {HTMLIFrameElement} */ (element).contentDocument?.getElementById('caduceus-screenshot-mode-styles'),
  ))).toBe(false)

  await page.evaluate(() => window.dispatchEvent(new CustomEvent('caduceus:shortcut:core:toggle-screenshot-mode')))
  await expect(page.locator('html')).toHaveAttribute('data-screenshot-mode', 'true')

  await page.reload()
  await waitForUiReady(page)
  await expect(page.locator('html')).toHaveAttribute('data-screenshot-mode', 'true')
  await page.getByRole('button', { name: 'Exit screenshot mode' }).click()
  await expect(page.locator('html')).not.toHaveAttribute('data-screenshot-mode', 'true')
  expect(await page.evaluate(() => sessionStorage.getItem('caduceus-screenshot-mode'))).toBeNull()
  const restoredPrivacy = await page.locator('.email-subject-text').first().evaluate(element => {
    const style = getComputedStyle(element)
    return { color: style.color, textSecurity: style.webkitTextSecurity }
  })
  expect(restoredPrivacy.textSecurity).not.toBe('disc')
  expect(restoredPrivacy.color).not.toBe('rgba(0, 0, 0, 0)')
})

test('does not strand an unauthenticated login screen in screenshot mode', async ({ page }) => {
  await useTestApi(page)
  await page.addInitScript(() => sessionStorage.setItem('caduceus-screenshot-mode', 'on'))
  await page.goto('/')
  await expect(page.getByText('Sign in')).toBeVisible()
  await expect(page.locator('html')).not.toHaveAttribute('data-screenshot-mode', 'true')
  expect(await page.evaluate(() => sessionStorage.getItem('caduceus-screenshot-mode'))).toBeNull()
})
