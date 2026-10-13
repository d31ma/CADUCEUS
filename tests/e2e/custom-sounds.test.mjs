import { test, expect } from '@playwright/test'
import {
  API,
  captureUiErrors,
  deliverEmail,
  getToken,
  loginAs,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('previews, persists, mutes, and plays send lifecycle sound cues', async ({ page }) => {
  const domain = `sounds-${Date.now()}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const account = await getToken(email)
  await page.addInitScript(() => {
    try {
      window.__caduceusSoundCalls = JSON.parse(sessionStorage.getItem('caduceus_e2e_sound_calls') || '[]')
    } catch { window.__caduceusSoundCalls = [] }
    class FakeAudioContext {
      state = 'running'
      currentTime = 1
      destination = {}
      createOscillator() {
        return {
          type: 'sine',
          frequency: { setValueAtTime() {} },
          connect() {}, start() {}, stop() {},
        }
      }
      createGain() {
        return {
          gain: { setValueAtTime() {}, exponentialRampToValueAtTime() {} },
          connect() {},
        }
      }
      async resume() {}
    }
    window.AudioContext = FakeAudioContext
    window.addEventListener('caduceus:sound-played', event => {
      window.__caduceusSoundCalls.push(event.detail)
      sessionStorage.setItem('caduceus_e2e_sound_calls', JSON.stringify(window.__caduceusSoundCalls))
    })
  })
  await useTestApi(page)
  await loginAs(page, account.token, account.email, account.role, account.domains)
  const errors = captureUiErrors(page)

  await page.goto('/settings')
  await waitForUiReady(page)
  const newMail = page.locator('[data-sound-event="new-mail"]')
  const confirm = page.locator('[data-sound-event="confirm"]')
  await expect(newMail.locator('select')).toHaveValue('chime')

  const bellSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.notificationSounds?.['new-mail'] === 'bell')
  await newMail.locator('select').selectOption('bell')
  expect((await bellSaved).status()).toBe(200)
  await newMail.getByRole('button', { name: 'Preview' }).click()
  await expect.poll(() => page.evaluate(() => window.__caduceusSoundCalls)).toContainEqual({ event: 'new-mail', soundId: 'bell' })

  const mutedSaved = page.waitForResponse(response =>
    response.url().endsWith('/users/me/preferences') &&
    response.request().method() === 'PUT' &&
    response.request().postDataJSON()?.notificationSounds?.confirm === 'none')
  await confirm.locator('select').selectOption('none')
  expect((await mutedSaved).status()).toBe(200)
  await expect(confirm.getByRole('button', { name: 'Preview' })).toBeDisabled()

  const persisted = await fetch(`${API}/users/me/preferences`, {
    headers: { Authorization: `Bearer ${account.token}` },
  }).then(response => response.json())
  expect(persisted.notificationSounds).toMatchObject({ 'new-mail': 'bell', confirm: 'none' })

  await page.reload()
  await waitForUiReady(page)
  await expect(page.locator('[data-sound-event="new-mail"] select')).toHaveValue('bell')
  await expect(page.locator('[data-sound-event="confirm"] select')).toHaveValue('none')

  await page.goto('/inbox')
  await waitForUiReady(page)
  await deliverEmail({
    recipient: account.email,
    sender: 'sound-source@outside.test',
    subject: `Foreground sound ${Date.now()}`,
    body: 'Play the selected new-mail cue.',
  })
  await expect.poll(() => page.evaluate(() => window.__caduceusSoundCalls), { timeout: 10_000 })
    .toContainEqual({ event: 'new-mail', soundId: 'bell' })

  await page.goto('/compose')
  await waitForUiReady(page)
  await page.locator('[data-compose-to] input').fill('recipient@outside.test')
  await page.locator('[data-compose-subject] input').fill('Sound lifecycle')
  await page.locator('[data-compose-editor]').fill('Sound test body')
  await page.route(`${API}/send`, route => route.fulfill({
    status: 200,
    contentType: 'application/json',
    body: JSON.stringify({ sent: true, undoId: 'sound-undo' }),
  }))
  await page.getByRole('button', { name: 'Send', exact: true }).click()
  await expect.poll(() => page.evaluate(() => window.__caduceusSoundCalls)).toEqual(expect.arrayContaining([
    { event: 'hit-send', soundId: 'pop' },
    { event: 'send', soundId: 'pulse' },
  ]))
  expect(errors).toEqual([])
})
