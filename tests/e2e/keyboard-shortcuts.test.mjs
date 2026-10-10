import { expect, test } from '@playwright/test'
import { API, deliverEmail, getToken, loginAs, seedDomain, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

async function pressMod(page, key) {
  const modifier = await page.evaluate(() => /Mac|iPhone|iPad/i.test(navigator.platform) ? 'Meta' : 'Control')
  await page.keyboard.press(`${modifier}+${key}`)
}

async function authenticate(page) {
  const domain = `keys-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain] })
  const identity = await getToken(email)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  return { email, token: identity.token, domain }
}

async function goto(page, path) {
  await page.goto(path)
  await waitForUiReady(page)
  await page.waitForFunction(() => window._caduceusKeyboard?.getCatalog?.().length > 0)
  await page.waitForFunction(() => window._caduceusPreferencesReady === true)
}

async function countShortcutEvents(page) {
  await page.evaluate(() => {
    window.__shortcutCounts = {}
    for (const action of window._caduceusKeyboard.getCatalog()) {
      window.addEventListener(`caduceus:shortcut:${action.id}`, () => {
        window.__shortcutCounts[action.id] = (window.__shortcutCounts[action.id] || 0) + 1
      })
    }
  })
}

async function shortcutCounts(page) {
  return page.evaluate(() => ({ ...window.__shortcutCounts }))
}

test('discovers current shortcuts accessibly and suppresses commands while typing', async ({ page }) => {
  await authenticate(page)
  await goto(page, '/inbox')

  const returnTarget = page.getByRole('link', { name: 'CADUCEUS' })
  await returnTarget.focus()
  await page.keyboard.press('?')
  const dialog = page.getByRole('dialog', { name: 'Keyboard shortcuts' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByText('Calendar', { exact: true })).toBeVisible()
  await expect(dialog.getByText('Reply all', { exact: true })).toBeVisible()
  await expect(dialog.getByText(/^(Shift\+R|⇧R)$/).first()).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(returnTarget).toBeFocused()

  const search = page.locator('[data-thread-search-input]')
  await search.focus()
  await page.keyboard.press('?')
  await expect(page.getByRole('dialog', { name: 'Keyboard shortcuts' })).toHaveCount(0)
  await expect(search).toHaveValue('?')
})

test('dispatches one deterministic action for shared mailbox and reader bindings', async ({ page }) => {
  const auth = await authenticate(page)
  await seedDomain(auth.token, auth.domain)

  await goto(page, '/inbox')
  await countShortcutEvents(page)
  await page.keyboard.press('Shift+R')
  await expect.poll(() => shortcutCounts(page)).toMatchObject({ 'core:refresh': 1 })
  expect((await shortcutCounts(page))['email:reply-all'] || 0).toBe(0)

  const delivered = await deliverEmail({
    recipient: auth.email,
    sender: 'sender@other.test',
    subject: `Scoped shortcut ${Date.now()}`,
    body: 'One binding must have one meaning.',
    messageId: `scoped-${Date.now()}`,
  })

  await goto(page, `/email/${encodeURIComponent(delivered.emailId)}`)
  await countShortcutEvents(page)
  await page.keyboard.press('Shift+R')
  await expect.poll(() => shortcutCounts(page)).toMatchObject({ 'email:reply-all': 1 })
  expect((await shortcutCounts(page))['core:refresh'] || 0).toBe(0)
})

test('persists customization, disables the old binding, and reports conflicts', async ({ page }) => {
  const auth = await authenticate(page)
  await goto(page, '/settings#keyboard-shortcuts')

  const composeBinding = page.locator('[data-shortcut-action="core:compose"]')
  await expect(composeBinding).toHaveValue('c')
  await expect(composeBinding).toBeEnabled()
  await composeBinding.fill('mod+k')
  await expect(composeBinding).toHaveValue('mod+k')
  await page.getByRole('button', { name: 'Save shortcuts' }).click()
  await expect(page.locator('.shortcut-settings-status')).toContainText('Keyboard shortcuts saved.')
  const saved = await fetch(`${API}/users/me/preferences`, { headers: { Authorization: `Bearer ${auth.token}` } })
  expect(saved.ok).toBe(true)
  expect((await saved.json()).keyboardShortcutOverrides).toEqual({ 'core:compose': 'mod+k' })

  await goto(page, '/inbox')
  await expect.poll(() => page.evaluate(() => window._caduceusKeyboard?.getBindings?.()['core:compose'])).toBe('mod+k')
  await page.keyboard.press('c')
  await expect(page).toHaveURL(/\/inbox\/?$/)
  await pressMod(page, 'K')
  await expect(page).toHaveURL(/\/compose\/?$/)

  await goto(page, '/settings#keyboard-shortcuts')
  await expect(page.locator('[data-shortcut-action="core:compose"]')).toHaveValue('mod+k')
  await page.locator('[data-shortcut-action="core:compose"]').fill('mod+1')
  await page.getByRole('button', { name: 'Save shortcuts' }).click()
  await expect(page.getByRole('alert')).toContainText('both use')
})

test('runs the complete calendar view, navigation, refresh, move, and resize command surface', async ({ page }) => {
  const auth = await authenticate(page)
  const initialDay = new Date()
  const start = new Date(Date.now() + 5 * 60 * 1000)
  start.setSeconds(0, 0)
  const end = new Date(start.getTime() + 60 * 60 * 1000)
  const title = `Keyboard calendar ${Date.now()}`
  const created = await fetch(`${API}/calendar/events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${auth.token}` },
    body: JSON.stringify({ title, start: start.toISOString(), end: end.toISOString() }),
  })
  expect(created.ok).toBe(true)
  const listed = await fetch(`${API}/calendar/events`, { headers: { Authorization: `Bearer ${auth.token}` } })
  expect(listed.ok).toBe(true)
  expect((await listed.json()).some(event => event.title === title)).toBe(true)

  await goto(page, '/calendar')
  await expect.poll(() => page.evaluate(() => window._caduceusKeyboard?.getContext?.())).toBe('calendar')
  await pressMod(page, '1')
  await expect(page.getByRole('tab', { name: 'Day', exact: true })).toHaveClass(/active/)
  if (start.getFullYear() !== initialDay.getFullYear() || start.getMonth() !== initialDay.getMonth() || start.getDate() !== initialDay.getDate()) {
    await page.getByRole('button', { name: 'Next month' }).click()
  }
  const event = page.getByRole('button', { name: new RegExp(title) }).first()
  await expect(event).toBeVisible()
  await event.focus()
  await expect(event).toHaveClass(/keyboard-selected/)

  await page.keyboard.press('ArrowDown')
  await expect(page.locator('.shell-toast-message')).toContainText('Event moved.')
  await page.keyboard.press('Shift+ArrowDown')
  await expect(page.locator('.shell-toast-message')).toContainText('Event resized.')

  await pressMod(page, '3')
  await expect(page.getByRole('tab', { name: 'Month', exact: true })).toHaveClass(/active/)
  const before = await page.locator('.calendar-title').textContent()
  await pressMod(page, 'ArrowRight')
  await expect.poll(() => page.locator('.calendar-title').textContent()).not.toBe(before)
  await pressMod(page, 'T')
  await pressMod(page, 'Shift+R')
  await expect(page.locator('.calendar-page')).toBeVisible()
})
