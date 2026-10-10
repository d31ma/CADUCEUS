import { test, expect } from '@playwright/test'
import {
  API,
  deliverEmail,
  getToken,
  loginAs,
  seedDomain,
  seedUser,
  useTestApi,
  waitForUiReady,
} from './helpers.mjs'

test('snoozes a conversation with presets, labels it, and supports undo', async ({ page }) => {
  const domain = `snooze-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  const subject = `Snooze browser ${Date.now()}`
  await deliverEmail({ recipient: email, sender: 'projects@example.net', subject, body: 'Review tomorrow', messageId: `snooze-${Date.now()}` })
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)

  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText(subject, { exact: true }).first().click()
  await page.getByRole('button', { name: 'Snooze', exact: true }).click()

  const dialog = page.getByRole('dialog', { name: 'Snooze conversation' })
  await expect(dialog).toBeVisible()
  for (const label of ['Later Today', 'Tonight', 'Tomorrow', 'This Weekend', 'Next Week', 'Next Month']) {
    await expect(dialog.getByRole('button', { name: label, exact: true })).toBeVisible()
  }
  await expect(dialog.locator('input[type="datetime-local"]')).toBeVisible()
  await dialog.getByRole('button', { name: 'Tomorrow', exact: true }).click()

  await expect(page.locator('#caduceus-toast-action')).toHaveText('Undo')
  const active = await fetch(`${API}/snooze`, { headers: { Authorization: `Bearer ${identity.token}` } }).then(response => response.json())
  expect(active).toHaveLength(1)
  expect(active[0]).toMatchObject({ subject, label: 'Tomorrow', count: 1 })

  await page.locator(`a[data-folder="snoozed:${domain}"]`).click()
  await expect(page.getByText(subject, { exact: true }).first()).toBeVisible()
  await expect(page.locator('.email-snooze-badge').first()).toContainText('Snoozed until')

  await page.locator('#caduceus-toast-action').click()
  await expect(page.getByText(subject, { exact: true })).toHaveCount(0)
  await expect.poll(async () => {
    const rows = await fetch(`${API}/snooze`, { headers: { Authorization: `Bearer ${identity.token}` } }).then(response => response.json())
    return rows.length
  }).toBe(0)
})

test('queues an encrypted snooze and replays its client request exactly once', async ({ page }) => {
  const domain = `offline-snooze-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `admin@${domain}`
  await seedUser(email, { domains: [domain], role: 'admin' })
  const identity = await getToken(email)
  await seedDomain(identity.token, domain)
  const subject = `Offline snooze ${Date.now()}`
  const delivered = await deliverEmail({
    recipient: email, sender: 'projects@example.net', subject,
    body: 'Snooze this after reconnect.', messageId: `offline-snooze-${Date.now()}`,
  })
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  let attempts = 0
  let payload
  await page.route(`${API}/snooze`, async route => {
    if (route.request().method() !== 'POST') return route.continue()
    attempts += 1
    payload = route.request().postDataJSON()
    if (attempts === 1) return route.abort('failed')
    return route.continue()
  })
  await page.goto('/inbox')
  await waitForUiReady(page)
  await page.getByText(subject, { exact: true }).first().click()
  await page.getByRole('button', { name: 'Snooze', exact: true }).click()
  const navigated = page.waitForNavigation({ waitUntil: 'load' })
  await page.getByRole('dialog', { name: 'Snooze conversation' })
    .getByRole('button', { name: 'Tomorrow', exact: true }).click()
  await navigated
  await waitForUiReady(page)
  expect(payload.clientRequestId).toBeTruthy()

  const encrypted = await page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve('')
    request.onsuccess = () => {
      const read = request.result.transaction('writes', 'readonly').objectStore('writes').getAll()
      read.onerror = () => resolve('')
      read.onsuccess = () => resolve(JSON.stringify(read.result))
    }
  }))
  expect(encrypted).not.toContain(delivered.emailId)

  const synchronized = page.waitForResponse(response => response.url().endsWith('/snooze')
    && response.request().method() === 'POST' && response.ok())
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  const accepted = await synchronized.then(response => response.json())
  expect(attempts).toBe(2)
  const replay = await fetch(`${API}/snooze`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${identity.token}` },
    body: JSON.stringify(payload),
  }).then(response => response.json())
  expect(replay).toEqual(accepted)
  const active = await fetch(`${API}/snooze`, {
    headers: { Authorization: `Bearer ${identity.token}` },
  }).then(response => response.json())
  expect(active.filter(item => item.subject === subject)).toHaveLength(1)
})
