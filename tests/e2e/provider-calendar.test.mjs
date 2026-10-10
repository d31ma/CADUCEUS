import { expect, test } from '@playwright/test'
import { captureUiErrors, getToken, loginAs, seedUser, useTestApi, waitForUiReady } from './helpers.mjs'

test('renders connected-calendar events as writable without losing provider identity', async ({ page }) => {
  const email = `provider-calendar-${Date.now()}@example.com`
  await seedUser(email, { domains: ['example.com'], role: 'admin' })
  const identity = await getToken(email)
  await useTestApi(page)
  await loginAs(page, identity.token, identity.email, identity.role, identity.domains)
  const errors = captureUiErrors(page)

  const start = new Date()
  start.setDate(start.getDate() + 1)
  start.setHours(10, 0, 0, 0)
  const end = new Date(start)
  end.setHours(11)
  const title = `Provider planning ${Date.now()}`
  const providerEvent = {
    id: 'provider-calendar-event', title, start: start.toISOString(), end: end.toISOString(), allDay: false,
    location: 'Remote room', description: '', attendees: [], organizer: 'owner@remote.example',
    source: 'provider', icsUid: 'provider-uid', recurrence: '', recurrenceDates: [], exceptionDates: [],
    timeZone: 'UTC', seriesId: '', recurrenceId: '', isRecurring: false, isException: false, readOnly: false,
    reminderMinutes: null, status: 'confirmed', accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', accountLabel: 'Work',
    accountColor: '#336699', calendarId: 'calendar_work', calendarName: 'Team calendar', calendarColor: '#112233',
    providerResourceId: 'caldav_resource', createdAt: start.toISOString(), updatedAt: start.toISOString(),
  }

  await page.goto('/calendar')
  await waitForUiReady(page)
  await page.evaluate(async event => {
    const calendar = window.__tachyonTac?.instance
    if (!calendar) throw new Error('calendar component is unavailable')
    calendar.$events = [event]
    await calendar.renderCurrentState()
  }, providerEvent)
  const chip = page.locator('[data-calendar-event-id="provider-calendar-event"]')
  await expect(chip).toBeVisible()
  await expect(chip).toHaveClass(/calendar-event-chip-provider/)
  expect(await page.evaluate(event => ({
    draggable: window.__tachyonTac.instance.eventDraggable(event),
    readOnly: window.__tachyonTac.instance.isReadOnlyEvent(event),
    domDraggable: document.querySelector('[data-calendar-event-id="provider-calendar-event"]')?.draggable,
    attribute: document.querySelector('[data-calendar-event-id="provider-calendar-event"]')?.getAttribute('draggable'),
  }), providerEvent)).toEqual({ draggable: true, readOnly: false, domDraggable: true, attribute: 'true' })
  await page.evaluate(async event => window.__tachyonTac.instance.onEventClick(null, event), providerEvent)

  const dialog = page.getByRole('dialog')
  await expect(dialog.getByLabel('Title')).toHaveValue(title)
  await expect(dialog.getByLabel('Location')).toHaveValue('Remote room')
  await expect(dialog.getByRole('button', { name: 'Save', exact: true })).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Delete', exact: true })).toBeVisible()
  await dialog.getByRole('button', { name: 'Cancel' }).click()

  await page.evaluate(async calendar => {
    const component = window.__tachyonTac.instance
    component.$providerCalendars = [calendar]
    await component.openCreateModal(null)
  }, { accountId: providerEvent.accountId, accountLabel: providerEvent.accountLabel, accountColor: providerEvent.accountColor, calendarId: providerEvent.calendarId, calendarName: providerEvent.calendarName, calendarColor: providerEvent.calendarColor, writable: true })
  const createDialog = page.getByRole('dialog')
  await expect(createDialog.getByLabel('Calendar')).toContainText('Work · Team calendar')
  await createDialog.getByLabel('Calendar').selectOption(providerEvent.calendarId)
  await expect(createDialog.getByLabel('Calendar')).toHaveValue(providerEvent.calendarId)
  expect(errors).toEqual([])
})
