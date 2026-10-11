/**
 * E2E smoke tests — verifies the preview + API servers start and respond.
 *
 * Full login/app interaction tests are deferred until the WebAuthn/Passkey
 * UI is stabilized and a headed-browser debugging pass can be done.
 */
import { test, expect } from '@playwright/test'
import { API, useTestApi } from './helpers.mjs'

test('preview server loads the login page', async ({ page }) => {
  await useTestApi(page)
  await page.goto('/')
  // The page should render the CADUCEUS shell (SPA renderer loads)
  await expect(page.locator('body')).not.toBeEmpty()
})

test('API server responds to health-like request', async ({ page }) => {
  // The preview server proxies API requests or the page loads API-bound config
  await useTestApi(page)
  await page.goto('/')
  // One origin: the page is served by the API server and calls it on its own origin
  const config = await page.evaluate(() => [location.origin, window.CADUCEUS_CONFIG?.apiUrl])
  expect(config).toEqual([API, ''])
})
