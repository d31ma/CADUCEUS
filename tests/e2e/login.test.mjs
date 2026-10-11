/** Sign-in UI against the real API and the real identity engine. */
import { test, expect } from '@playwright/test'
import { API, uniqueEmail, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

const field = (page, attribute) => page.locator(`w-text-field[${attribute}] input`)

test('shows the provider-free sign-in entry point', async ({ page }) => {
  await useTestApi(page)
  await page.goto('/')
  await waitForUiReady(page, { shell: false })
  // One origin: the client calls the API on the page's own origin.
  expect(await page.evaluate(() => [location.origin, window.CADUCEUS_CONFIG.apiUrl])).toEqual([API, ''])
  await expect(page.getByText('Sign in')).toBeVisible()
  await expect(field(page, 'data-login-email')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Use a recovery code' })).toBeVisible()
})

test('does not disclose unknown accounts', async ({ page }) => {
  await useTestApi(page)
  await page.goto('/')
  await waitForUiReady(page, { shell: false })
  await field(page, 'data-login-email').fill('nobody@example.com')
  const responsePromise = page.waitForResponse(response => response.url().includes('/auth/login/begin'))
  await page.getByRole('button', { name: 'Continue' }).click()

  // The server answers an unknown address exactly as it answers a known one,
  // so the UI reaches the same step either way and reveals nothing.
  expect((await responsePromise).status()).toBe(200)
})

test('an account with no passkey is offered its password first', async ({ page }) => {
  const email = uniqueEmail()
  await seedUser(email)
  await useTestApi(page)
  await page.goto('/')
  await waitForUiReady(page, { shell: false })
  await field(page, 'data-login-email').fill(email)
  const responsePromise = page.waitForResponse(response => response.url().includes('/auth/login/begin'))
  await page.getByRole('button', { name: 'Continue' }).click()
  expect((await responsePromise).status()).toBe(200)

  // Recovery codes are secondary factors in SESAME. A freshly provisioned
  // mailbox has no passkey, so the client must establish the password factor
  // before it offers recovery as the second factor.
  await expect(field(page, 'data-login-password')).toBeVisible({ timeout: 15_000 })
})

test('signs in with a password and a recovery code', async ({ page }) => {
  const email = uniqueEmail()
  const seeded = await seedUser(email)
  await useTestApi(page)
  await page.goto('/')
  await waitForUiReady(page, { shell: false })

  await field(page, 'data-login-email').fill(email)
  await page.getByRole('button', { name: 'Continue' }).click()

  // The deployment's policy decides which factors appear; the client only
  // answers what the server asked for.
  const password = field(page, 'data-login-password')
  await expect(password).toBeVisible({ timeout: 15_000 })
  await password.fill(seeded.password)
  await page.getByRole('button', { name: 'Continue' }).first().click()

  // A newly provisioned mailbox has no passkey, so the second factor it holds
  // is a recovery code.
  const recovery = field(page, 'data-login-recovery-code')
  await expect(recovery).toBeVisible({ timeout: 15_000 })
  await recovery.fill(seeded.recoveryCodes[0])
  await page.getByRole('button', { name: 'Verify and sign in' }).first().click()

  await page.waitForURL(/\/inbox/, { timeout: 30_000 })
  const stored = await page.evaluate(() => ({
    token: sessionStorage.getItem('caduceus_token'),
    email: sessionStorage.getItem('caduceus_email'),
  }))
  expect(stored.email).toBe(email)
  expect(stored.token?.split('.').length).toBe(3)
})

test('recovery view switches between saved and administrator codes', async ({ page }) => {
  await useTestApi(page)
  await page.goto('/')
  await waitForUiReady(page, { shell: false })
  await page.getByRole('button', { name: 'Use a recovery code' }).click()
  await expect(field(page, 'data-login-recovery-code')).toBeVisible()
  await page.getByRole('button', { name: 'Use an administrator code' }).click()
  await expect(field(page, 'data-login-admin-code')).toBeVisible()
})
