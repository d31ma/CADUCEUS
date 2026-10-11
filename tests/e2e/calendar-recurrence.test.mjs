import { expect, test } from '@playwright/test'
import { API, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

test.setTimeout(60_000)

async function authenticate(page) {
  const domain = `recurrence-${Date.now()}-${Math.floor(Math.random() * 1_000_000)}.test`
  const email = `owner@${domain}`
  await seedUser(email, { domains: [domain] })
  const identity = await getToken(email)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  return { token: identity.token }
}

async function gotoCalendar(page) {
  await page.goto('/calendar')
  await waitForUiReady(page)
  await expect(page.locator('.calendar-page')).toBeVisible()
}

function localDate(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

function localDateTime(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${localDate(date)}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

test('creates, edits, and deletes recurring occurrences with explicit series scope', async ({ page }) => {
  const auth = await authenticate(page)
  await gotoCalendar(page)

  const title = `Recurring planning ${Date.now()}`
  await page.getByRole('button', { name: 'New event' }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await dialog.getByLabel('Title').fill(title)
  await dialog.getByLabel('Repeat').selectOption('weekly')
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Event created.')

  const occurrences = page.locator('[data-calendar-event-id]').filter({ hasText: title })
  await expect.poll(() => occurrences.count()).toBeGreaterThan(1)
  await expect(occurrences.first().locator('.calendar-recurrence-indicator')).toBeVisible()

  await occurrences.nth(1).click()
  await expect(dialog.getByLabel('Apply changes to')).toHaveValue('instance')
  await expect(dialog.getByText('Only this occurrence will change')).toBeVisible()
  const movedTitle = `${title} moved`
  await dialog.getByLabel('Title').fill(movedTitle)
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Occurrence saved.')
  await expect(page.locator('[data-calendar-event-id]').filter({ hasText: movedTitle })).toHaveCount(1)
  await expect(page.locator('[data-calendar-event-id]').filter({ hasText: title }).filter({ hasNotText: movedTitle }).first()).toBeVisible()

  await page.locator('[data-calendar-event-id]').filter({ hasText: movedTitle }).click()
  page.once('dialog', confirmation => confirmation.accept())
  await dialog.getByRole('button', { name: 'Delete' }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Occurrence deleted. The series remains.')
  await expect(page.locator('[data-calendar-event-id]').filter({ hasText: movedTitle })).toHaveCount(0)
  await expect(page.locator('[data-calendar-event-id]').filter({ hasText: title }).first()).toBeVisible()

  await page.locator('[data-calendar-event-id]').filter({ hasText: title }).nth(1).click()
  await dialog.getByLabel('Apply changes to').selectOption('series')
  await expect(dialog.getByText('Changes apply to every occurrence')).toBeVisible()
  const seriesTitle = `${title} series`
  const seriesStartValue = await dialog.getByLabel('Start').inputValue()
  const seriesEndValue = await dialog.getByLabel('End').inputValue()
  expect(seriesStartValue.slice(0, 10)).not.toBe('')
  const resizedEnd = new Date(seriesEndValue)
  resizedEnd.setHours(resizedEnd.getHours() + 1)
  await dialog.getByLabel('Title').fill(seriesTitle)
  await dialog.getByLabel('End').fill(localDateTime(resizedEnd))
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Event series saved.')
  const renamed = page.locator('[data-calendar-event-id]').filter({ hasText: seriesTitle })
  await expect.poll(() => renamed.count()).toBeGreaterThan(1)

  const response = await fetch(`${API}/calendar/events?start=${encodeURIComponent(new Date(new Date().getFullYear(), new Date().getMonth() - 1, 1).toISOString())}&end=${encodeURIComponent(new Date(new Date().getFullYear(), new Date().getMonth() + 2, 1).toISOString())}`, {
    headers: { Authorization: `Bearer ${auth.token}` },
  })
  expect(response.ok).toBe(true)
  const persisted = await response.json()
  expect(persisted.filter(event => event.title === seriesTitle).length).toBeGreaterThan(1)
  expect(persisted.some(event => event.isRecurring && event.seriesId && event.recurrenceId)).toBe(true)
  expect(persisted.filter(event => event.title === seriesTitle)
    .every(event => Date.parse(event.end) - Date.parse(event.start) === 2 * 60 * 60_000)).toBe(true)
})

test('traps focus in the event dialog and restores it after Escape', async ({ page }) => {
  await authenticate(page)
  await gotoCalendar(page)
  const trigger = page.getByRole('button', { name: 'New event' })
  await trigger.focus()
  await trigger.click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await expect.poll(() => page.evaluate(() => !!document.activeElement?.closest?.('.event-modal'))).toBe(true)
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(trigger).toBeFocused()
})

test('switches views, navigates months, and creates from the quick-event popover', async ({ page }) => {
  await authenticate(page)
  await gotoCalendar(page)

  await page.getByRole('tab', { name: 'Day', exact: true }).click()
  await expect(page.locator('.calendar-agenda-grid-day')).toBeVisible()

  await page.getByRole('tab', { name: 'Month', exact: true }).click()
  await expect(page.locator('.calendar-month-grid')).toBeVisible()
  const before = await page.locator('.calendar-title').textContent()
  await page.getByRole('button', { name: 'Next month' }).click()
  await expect.poll(() => page.locator('.calendar-title').textContent()).not.toBe(before)

  const cell = page.locator('.calendar-cell:not(.calendar-cell-other)').first()
  const selectedDate = await cell.getAttribute('data-calendar-date')
  expect(selectedDate).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  await cell.locator('.calendar-cell-header').click()

  const quick = page.locator('.quick-event-popover')
  await expect(quick).toBeVisible()
  await expect(quick).toContainText(new Date(`${selectedDate}T12:00:00`).toLocaleDateString([], { weekday: 'long', month: 'long', day: 'numeric' }))
  const quickTitle = `Quick planning ${Date.now()}`
  await quick.locator('input').fill(quickTitle)
  await quick.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Event created.')
  await expect(page.locator(`[data-calendar-date="${selectedDate}"]`).getByText(quickTitle, { exact: true })).toBeVisible()

  await cell.locator('.calendar-cell-header').click()
  await quick.getByRole('button', { name: 'More options' }).click()
  const dialog = page.locator('.event-modal')
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('Start')).toHaveValue(new RegExp(`^${selectedDate}T09:00`))
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(dialog).toHaveCount(0)
})

test('queues an encrypted calendar create and replays it idempotently after reconnect', async ({ page }) => {
  const auth = await authenticate(page)
  let attempts = 0
  let payload
  await page.route(`${API}/calendar/events`, async route => {
    if (route.request().method() !== 'POST') return route.continue()
    attempts += 1
    payload = route.request().postDataJSON()
    if (attempts === 1) return route.abort('failed')
    return route.continue()
  })
  await gotoCalendar(page)
  const cell = page.locator('.calendar-cell:not(.calendar-cell-other)').first()
  await cell.locator('.calendar-cell-header').click()
  const title = `Offline calendar ${Date.now()}`
  const quick = page.locator('.quick-event-popover')
  await expect(quick).toBeVisible()
  await quick.locator('input').fill(title)
  await quick.getByRole('button', { name: 'Create', exact: true }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Event creation queued.')
  expect(payload.idempotencyKey).toBeTruthy()

  const encrypted = await page.evaluate(async () => await new Promise(resolve => {
    const request = indexedDB.open('caduceus-offline-write-vault')
    request.onerror = () => resolve('')
    request.onsuccess = () => {
      const read = request.result.transaction('writes', 'readonly').objectStore('writes').getAll()
      read.onerror = () => resolve('')
      read.onsuccess = () => resolve(JSON.stringify(read.result))
    }
  }))
  expect(encrypted).not.toContain(title)

  const synchronized = page.waitForResponse(response => response.url().endsWith('/calendar/events')
    && response.request().method() === 'POST' && response.ok())
  await page.evaluate(() => window.dispatchEvent(new Event('online')))
  const accepted = await synchronized.then(response => response.json())
  await expect(page.locator('[data-calendar-event-id]').filter({ hasText: title })).toBeVisible()
  expect(attempts).toBe(2)

  const replay = await fetch(`${API}/calendar/events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${auth.token}` },
    body: JSON.stringify(payload),
  }).then(response => response.json())
  expect(replay).toEqual(accepted)
  const events = await fetch(`${API}/calendar/events`, {
    headers: { Authorization: `Bearer ${auth.token}` },
  }).then(response => response.json())
  expect(events.filter(event => event.title === title)).toHaveLength(1)
})

test('dragging an owned event to another day preserves its time and duration', async ({ page }) => {
  const auth = await authenticate(page)
  const start = new Date(); start.setDate(start.getDate() + 1); start.setHours(14, 30, 0, 0)
  const end = new Date(start); end.setMinutes(end.getMinutes() + 45)
  const target = new Date(start); target.setDate(target.getDate() + 2)
  const title = `Dragged planning ${Date.now()}`
  const response = await fetch(`${API}/calendar/events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${auth.token}` },
    body: JSON.stringify({ title, start: start.toISOString(), end: end.toISOString(), timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone }),
  })
  expect(response.ok).toBe(true)
  const persistedBeforeRender = await fetch(`${API}/calendar/events`, { headers: { Authorization: `Bearer ${auth.token}` } }).then(result => result.json())
  expect(persistedBeforeRender.some(event => event.title === title)).toBe(true)

  await gotoCalendar(page)
  await page.evaluate(() => window.dispatchEvent(new CustomEvent('caduceus:shortcut:calendar:refresh-calendars')))
  const pad = value => String(value).padStart(2, '0')
  const targetDate = `${target.getFullYear()}-${pad(target.getMonth() + 1)}-${pad(target.getDate())}`
  const chip = page.locator('[data-calendar-event-id]').filter({ hasText: title })
  const targetCell = page.locator(`[data-calendar-date="${targetDate}"]`)
  await expect(chip).toBeVisible()
  await chip.dragTo(targetCell)
  await expect(page.locator('.shell-toast-message')).toContainText('Event moved.')
  await expect(targetCell.getByText(title, { exact: true })).toBeVisible()

  const moved = await fetch(`${API}/calendar/events`, { headers: { Authorization: `Bearer ${auth.token}` } }).then(result => result.json())
  const stored = moved.find(event => event.title === title)
  expect(new Date(stored.start).getHours()).toBe(14)
  expect(Date.parse(stored.end) - Date.parse(stored.start)).toBe(45 * 60 * 1000)
})

test('creates, edits, and date-space drags a multi-day all-day event', async ({ page }) => {
  const auth = await authenticate(page)
  await gotoCalendar(page)

  const startDate = new Date(); startDate.setDate(startDate.getDate() + 2)
  const endDate = new Date(startDate); endDate.setDate(endDate.getDate() + 3)
  const targetDate = new Date(startDate); targetDate.setDate(targetDate.getDate() + 8)
  const start = localDate(startDate)
  const end = localDate(endDate)
  const target = localDate(targetDate)
  const expectedTargetEnd = new Date(targetDate); expectedTargetEnd.setDate(expectedTargetEnd.getDate() + 3)
  const title = `All-day planning ${Date.now()}`

  await page.getByRole('button', { name: 'New event' }).click()
  const dialog = page.getByRole('dialog')
  await dialog.getByLabel('Title').fill(title)
  await dialog.getByLabel('All day').click()
  await expect(dialog.getByLabel('All day')).toBeChecked()
  await expect(dialog.getByLabel('Start')).toHaveAttribute('type', 'date')
  await dialog.getByLabel('Start').fill(start)
  await dialog.getByLabel('End').fill(end)
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect(page.locator('.shell-toast-message')).toContainText('Event created.')

  let persisted = await fetch(`${API}/calendar/events`, {
    headers: { Authorization: `Bearer ${auth.token}` },
  }).then(result => result.json())
  let stored = persisted.find(event => event.title === title)
  expect(stored).toMatchObject({ start, end, allDay: true })

  const chips = page.locator('[data-calendar-event-id]').filter({ hasText: title })
  await expect(chips).toHaveCount(3)
  await chips.first().click()
  await expect(dialog.getByLabel('All day')).toBeChecked()
  await expect(dialog.getByLabel('Start')).toHaveValue(start)
  await expect(dialog.getByLabel('End')).toHaveValue(end)
  await dialog.getByRole('button', { name: 'Cancel' }).click()

  // Re-entering the calendar proves the date-only representation survives a
  // full page lifecycle and removes transient modal/toast overlays from the
  // native drag transaction.
  await gotoCalendar(page)
  await page.evaluate(() => window.dispatchEvent(new CustomEvent('caduceus:shortcut:calendar:refresh-calendars')))
  const targetCell = page.locator(`[data-calendar-date="${target}"]`)
  const sourceChip = page.locator(`[data-calendar-date="${start}"] [data-calendar-event-id]`).filter({ hasText: title })
  await expect(sourceChip).toBeVisible()
  await expect(targetCell).toBeVisible()
  // Chromium can suppress pointer-synthesized HTML dragstart when the same
  // multi-day event has duplicate draggable chips. Dispatch the native DnD
  // transaction against the exact start-day chip and destination cell.
  await page.evaluate(({ start, target, title }) => {
    const source = [...document.querySelectorAll(`[data-calendar-date="${start}"] [data-calendar-event-id]`)]
      .find(node => node.textContent?.includes(title))
    const destination = document.querySelector(`[data-calendar-date="${target}"]`)
    if (!source || !destination) throw new Error('calendar drag endpoints are missing')
    const dataTransfer = new DataTransfer()
    source.dispatchEvent(new DragEvent('dragstart', { bubbles: true, cancelable: true, dataTransfer }))
    destination.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer }))
    destination.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer }))
    source.dispatchEvent(new DragEvent('dragend', { bubbles: true, cancelable: true, dataTransfer }))
  }, { start, target, title })
  await expect.poll(async () => {
    const events = await fetch(`${API}/calendar/events`, {
      headers: { Authorization: `Bearer ${auth.token}` },
    }).then(result => result.json())
    return events.find(event => event.title === title)?.start
  }).toBe(target)
  persisted = await fetch(`${API}/calendar/events`, {
    headers: { Authorization: `Bearer ${auth.token}` },
  }).then(result => result.json())
  stored = persisted.find(event => event.title === title)
  expect(stored).toMatchObject({
    start: target,
    end: localDate(expectedTargetEnd),
    allDay: true,
  })
})

test('an explicit all-day row converts a non-recurring timed event', async ({ page }) => {
  const auth = await authenticate(page)
  const start = new Date(); start.setDate(start.getDate() + 1); start.setHours(14, 30, 0, 0)
  const end = new Date(start); end.setMinutes(end.getMinutes() + 45)
  const date = localDate(start)
  const title = `All-day drop ${Date.now()}`
  const response = await fetch(`${API}/calendar/events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${auth.token}` },
    body: JSON.stringify({ title, start: start.toISOString(), end: end.toISOString(), timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone }),
  })
  expect(response.ok).toBe(true)

  await gotoCalendar(page)
  await page.getByRole('tab', { name: 'Day', exact: true }).click()
  await page.getByRole('button', { name: 'Next month' }).click()
  const chip = page.locator('[data-calendar-event-id]').filter({ hasText: title })
  const allDayRow = page.locator(`[data-calendar-all-day-date="${date}"]`)
  await expect(chip).toBeVisible()
  await expect(allDayRow).toBeVisible()
  await chip.dragTo(allDayRow)
  await expect(page.locator('.shell-toast-message')).toContainText('Event moved to all day.')

  const persisted = await fetch(`${API}/calendar/events`, {
    headers: { Authorization: `Bearer ${auth.token}` },
  }).then(result => result.json())
  expect(persisted.find(event => event.title === title)).toMatchObject({
    start: date,
    end: localDate(new Date(start.getFullYear(), start.getMonth(), start.getDate() + 1)),
    allDay: true,
  })
})

test('keeps the selected event visible and selected when switching to a narrow view', async ({ page }) => {
  const auth = await authenticate(page)
  const start = new Date(); start.setDate(start.getDate() + 4); start.setHours(16, 15, 0, 0)
  const end = new Date(start); end.setMinutes(end.getMinutes() + 45)
  const title = `Selected planning ${Date.now()}`
  const response = await fetch(`${API}/calendar/events`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${auth.token}` },
    body: JSON.stringify({ title, start: start.toISOString(), end: end.toISOString(), timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone }),
  })
  expect(response.ok).toBe(true)

  await gotoCalendar(page)
  await page.evaluate(() => window.dispatchEvent(new CustomEvent('caduceus:shortcut:calendar:refresh-calendars')))
  const event = page.locator('[data-calendar-event-id]').filter({ hasText: title }).first()
  await expect(event).toBeVisible()
  await event.click()
  await page.getByRole('dialog').getByRole('button', { name: 'Cancel' }).click()

  await page.getByRole('tab', { name: 'Day', exact: true }).click()
  const selected = page.locator('.calendar-agenda-grid-day [data-calendar-event-id]').filter({ hasText: title })
  await expect(selected).toBeVisible()
  await expect(selected).toHaveClass(/keyboard-selected/)
  await expect(selected).toBeInViewport()
  await expect(page.locator('.calendar-title')).toContainText(start.toLocaleDateString([], { month: 'long', day: 'numeric' }))
})
