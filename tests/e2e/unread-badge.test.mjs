import { test, expect } from '@playwright/test'
import { API, captureUiErrors, deliverEmail, getToken, loginAs, seedDomain, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function mailboxWithUnread(page, count, { nativeBadge }) {
  const domain = `badge-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  for (let index = 0; index < count; index++) {
    await deliverEmail({
      recipient: email,
      sender: `sender-${index}@outside.test`,
      subject: `Unread badge ${index + 1}`,
      body: `Message ${index + 1}`,
    })
  }
  await page.addInitScript(enabled => {
    window.__caduceusBadgeCalls = []
    Object.defineProperty(Navigator.prototype, 'setAppBadge', {
      configurable: true,
      value: enabled ? async function (value) { window.__caduceusBadgeCalls.push(['set', value]) } : undefined,
    })
    Object.defineProperty(Navigator.prototype, 'clearAppBadge', {
      configurable: true,
      value: enabled ? async function () { window.__caduceusBadgeCalls.push(['clear']) } : undefined,
    })
  }, nativeBadge)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  await page.goto('/inbox')
  await waitForUiReady(page)
}

test('keeps the installed-app badge synchronized as unread mail is read', async ({ page }) => {
  await mailboxWithUnread(page, 2, { nativeBadge: true })
  await expect.poll(() => page.evaluate(() => window.__caduceusBadgeCalls.at(-1))).toEqual(['set', 2])
  await expect(page).not.toHaveTitle(/^\(\d+\)/)

  await page.locator('.email-row').first().locator('.email-subject-text').click()
  await expect.poll(() => page.evaluate(() => window.__caduceusBadgeCalls.at(-1))).toEqual(['set', 1])
})

test('uses a live document-title count when the Badging API is unavailable', async ({ page }) => {
  await mailboxWithUnread(page, 2, { nativeBadge: false })
  await expect(page).toHaveTitle('(2) CADUCEUS')

  await page.locator('.email-row').first().locator('.email-subject-text').click()
  await expect(page).toHaveTitle('(1) CADUCEUS')

  await page.locator('.app-user-trigger').click()
  await page.getByRole('menuitem', { name: 'Screenshot mode' }).click()
  await expect(page).toHaveTitle('CADUCEUS')
  await page.getByRole('button', { name: 'Exit screenshot mode' }).click()
  await expect(page).toHaveTitle('(1) CADUCEUS')
})

test('persists all-account or focused-account unread badges through Settings', async ({ page }) => {
  const suffix = `${Date.now()}-${Math.floor(Math.random() * 1_000_000)}`
  const primaryDomain = `badge-primary-${suffix}.test`
  const focusedDomain = `badge-focused-${suffix}.test`
  const email = `owner@${primaryDomain}`
  await seedUser(email, { domains: [primaryDomain, focusedDomain], role: 'admin' })
  const account = await getToken(email)

  await page.addInitScript(() => {
    window.__caduceusBadgeCalls = []
    Object.defineProperty(Navigator.prototype, 'setAppBadge', {
      configurable: true,
      value: async function (value) { window.__caduceusBadgeCalls.push(['set', value]) },
    })
    Object.defineProperty(Navigator.prototype, 'clearAppBadge', {
      configurable: true,
      value: async function () { window.__caduceusBadgeCalls.push(['clear']) },
    })
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, [primaryDomain, focusedDomain])
  const errors = captureUiErrors(page)
  await page.goto('/settings')
  await waitForUiReady(page)

  const scope = page.locator('[data-unread-badge-scope]')
  await expect(scope).toHaveValue('all')
  await page.evaluate(({ primaryDomain, focusedDomain }) => {
    window.dispatchEvent(new CustomEvent('caduceus:folder-counts', {
      detail: {
        unreadInbox: 5,
        unreadInboxByDomain: { [primaryDomain]: 2, [focusedDomain]: 3 },
      },
    }))
  }, { primaryDomain, focusedDomain })
  await expect.poll(() => page.evaluate(() => window.__caduceusBadgeCalls.at(-1))).toEqual(['set', 5])

  const focusedSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.unreadBadgeScope === 'domain')
  await scope.selectOption('domain')
  expect((await focusedSaved).status()).toBe(200)

  const domain = page.locator('[data-unread-badge-domain]')
  await expect(domain).toBeVisible()
  const domainSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.unreadBadgeDomain === focusedDomain)
  await domain.selectOption(focusedDomain)
  expect((await domainSaved).status()).toBe(200)
  await page.evaluate(({ primaryDomain, focusedDomain }) => {
    window.dispatchEvent(new CustomEvent('caduceus:folder-counts', {
      detail: {
        unreadInbox: 5,
        unreadInboxByDomain: { [primaryDomain]: 2, [focusedDomain]: 3 },
      },
    }))
  }, { primaryDomain, focusedDomain })
  await expect.poll(() => page.evaluate(() => window.__caduceusBadgeCalls.at(-1))).toEqual(['set', 3])

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted).toMatchObject({
    unreadBadgeScope: 'domain',
    unreadBadgeDomain: focusedDomain,
  })

  await page.reload()
  await waitForUiReady(page)
  await expect(scope).toHaveValue('domain')
  await expect(domain).toHaveValue(focusedDomain)
  await expect.poll(async () => (await page.locator('.unread-badge-settings').boundingBox())?.width ?? Infinity)
    .toBeLessThanOrEqual(390)
  expect(errors).toEqual([])
})
