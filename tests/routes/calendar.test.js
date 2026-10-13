import { describe, it, expect, beforeAll, afterAll } from 'bun:test'
import { setupRoutes, teardownRoutes, token, req } from './helpers.js'
import { call } from '../support/controller-call.mjs'
import { Routes } from '../support/Routes.mjs'

const Events = Routes.CalendarEventsController
const Event = Routes.CalendarEventsIdController
const Respond = Routes.CalendarInvitesRespondController
const Team = Routes.CalendarTeamEventsController
const Calendars = Routes.CalendarCalendarsController

let app
const auth = () => token()
const other = () => token({ email: 'bob@example.com' })

const START = '2099-03-01T09:00:00.000Z'
const END = '2099-03-01T10:00:00.000Z'

beforeAll(async () => {
  app = await setupRoutes()
  app.providerCalendars = { available: () => false }
  await app.users.put({ email: 'alice@example.com', aliases: [], domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

const create = async (over = {}, as = auth()) => {
  const res = await call(Events, 'POST', req({ token: as, body: { title: 'Meeting', start: START, end: END, ...over } }))
  return res.id
}

describe('Events._validateTimes', () => {
  it('requires a title', () => {
    expect(Events._validateTimes({}).message).toMatch(/title/)
    expect(Events._validateTimes({ title: '   ' }).message).toMatch(/title/)
  })

  it('requires a parseable start and end', () => {
    expect(Events._validateTimes({ title: 'x' }).message).toMatch(/start/)
    expect(Events._validateTimes({ title: 'x', start: 'nope' }).message).toMatch(/start/)
    expect(Events._validateTimes({ title: 'x', start: START }).message).toMatch(/end/)
    expect(Events._validateTimes({ title: 'x', start: START, end: 'nope' }).message).toMatch(/end/)
  })

  it('rejects an end before the start', () => {
    expect(Events._validateTimes({ title: 'x', start: END, end: START }).message).toMatch(/on or after/)
  })

  it('trims and returns the three fields', () => {
    expect(Events._validateTimes({ title: ' x ', start: ` ${START} `, end: ` ${END} ` }))
      .toEqual({ title: 'x', start: START, end: END })
  })

  it('requires canonical dates and an exclusive end for all-day events', () => {
    expect(Events._validateTimes({ title: 'x', allDay: true, start: '2099-03-01', end: '2099-03-02' }))
      .toEqual({ title: 'x', start: '2099-03-01', end: '2099-03-02' })
    expect(Events._validateTimes({ title: 'x', allDay: true, start: START, end: END }).message).toMatch(/date-only/)
    expect(Events._validateTimes({ title: 'x', allDay: true, start: '2099-03-01', end: '2099-03-01' }).message)
      .toMatch(/after start/)
  })
})

describe('Events._normalizeAttendees', () => {
  it('returns an empty list for a non-array', () => {
    expect(Events._normalizeAttendees(undefined)).toEqual([])
    expect(Events._normalizeAttendees('bob')).toEqual([])
  })

  it('drops entries without an email', () => {
    expect(Events._normalizeAttendees([null, {}, { email: 5 }])).toEqual([])
  })

  it('lowercases the email and defaults name and status', () => {
    expect(Events._normalizeAttendees([{ email: 'BOB@X.COM' }]))
      .toEqual([{ email: 'bob@x.com', name: '', status: 'needsAction' }])
  })

  it('keeps a recognised status and name', () => {
    expect(Events._normalizeAttendees([{ email: 'b@x.com', name: 'B', status: 'declined' }]))
      .toEqual([{ email: 'b@x.com', name: 'B', status: 'declined' }])
  })

  it('replaces an unrecognised status', () => {
    expect(Events._normalizeAttendees([{ email: 'b@x.com', status: 'maybe' }])[0].status).toBe('needsAction')
  })
})

describe('POST /calendar/events', () => {
  it('requires auth', async () => {
    expect((await call(Events, 'POST', req({ body: {} }))).unauthorized).toBe(true)
  })

  it('400s on invalid input', async () => {
    const res = await call(Events, 'POST', req({ token: auth(), body: { title: '' } }))
    expect(res.error).toMatch(/title/)
  })

  it('creates an event with defaults', async () => {
    const id = await create()
    const res = await call(Event, 'GET', req({ token: auth(), paths: { id } }))
    expect(res.title).toBe('Meeting')
    expect(res.organizer).toBe('alice@example.com')
    expect(res.allDay).toBe(false)
    expect(res.reminderMinutes).toBeNull()
    expect(res.source).toBe('manual')
  })

  it('replays a local create idempotently after a lost response', async () => {
    const body = {
      title: 'Offline calendar create', start: START, end: END,
      idempotencyKey: 'calendar-offline-create-1',
    }
    const first = await call(Events, 'POST', req({ token: auth(), body }))
    const replay = await call(Events, 'POST', req({ token: auth(), body: { ...body } }))
    expect(replay).toEqual(first)
    expect((await app.calendarEvents.list('alice@example.com'))
      .filter(event => event.idempotencyKey === body.idempotencyKey)).toHaveLength(1)
  })

  it('keeps supplied optional fields', async () => {
    const id = await create({
      location: '  Room 2 ',
      description: 'notes',
      organizer: 'BOSS@example.com',
      recurrence: 'FREQ=WEEKLY',
      reminderMinutes: 10,
      attendees: [{ email: 'bob@example.com', status: 'accepted' }],
    })
    const res = await call(Event, 'GET', req({ token: auth(), paths: { id } }))
    expect(res.allDay).toBe(false)
    expect(res.location).toBe('Room 2')
    expect(res.organizer).toBe('boss@example.com')
    expect(res.recurrence).toBe('FREQ=WEEKLY')
    expect(res.reminderMinutes).toBe(10)
    expect(res.attendees[0].status).toBe('accepted')
  })

  it('persists all-day events as date-only exclusive ranges', async () => {
    const id = await create({ allDay: true, start: '2099-03-01', end: '2099-03-04' })
    const res = await call(Event, 'GET', req({ token: auth(), paths: { id } }))
    expect(res).toMatchObject({ allDay: true, start: '2099-03-01', end: '2099-03-04' })
  })
})

describe('GET /calendar/events', () => {
  it('requires auth', async () => {
    expect((await call(Events, 'GET', req())).unauthorized).toBe(true)
  })

  it('returns the caller events sorted by start', async () => {
    const res = await call(Events, 'GET', req({ token: auth(), query: {} }))
    const starts = res.map(e => e.start)
    expect(starts).toEqual([...starts].sort())
  })

  it('merges authenticated provider events as read-only calendar entries', async () => {
    let boundaryToken = ''
    app.providerCalendars = { available: () => true, list: async tokenValue => {
      boundaryToken = tokenValue
      return [{
        id: 'provider-calendar-event', userEmail: 'alice@remote.example', title: 'Provider meeting',
        start: START, end: END, source: 'provider', readOnly: true, icsUid: 'remote-uid',
        accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', accountLabel: 'Work', accountColor: '#336699',
        calendarId: 'calendar_work', calendarName: 'Work calendar', calendarColor: '#112233',
        providerResourceId: 'caldav_resource', createdAt: START, updatedAt: START,
      }]
    } }
    try {
      const callerToken = auth()
      const res = await call(Events, 'GET', req({ token: callerToken, query: {} }))
      const provider = res.find(event => event.id === 'provider-calendar-event')
      expect(boundaryToken).toBe(callerToken)
      expect(provider).toMatchObject({
        source: 'provider', readOnly: true, accountLabel: 'Work', calendarName: 'Work calendar',
      })
      expect(provider.rawICalendar).toBeUndefined()
    } finally {
      app.providerCalendars = { available: () => false }
    }
  })

  it('clips to a window', async () => {
    const res = await call(Events, 'GET', req({ token: auth(), query: { start: '2030-01-01T00:00:00.000Z', end: '2030-01-02T00:00:00.000Z' } }))
    expect(res).toEqual([])
  })

  it('expands recurring masters in a bounded range', async () => {
    const id = await create({ recurrence: 'FREQ=WEEKLY;COUNT=3', timeZone: 'UTC' })
    const res = await call(Events, 'GET', req({ token: auth(), query: {
      start: '2099-03-01T00:00:00.000Z',
      end: '2099-03-31T23:59:59.999Z',
    } }))
    const series = res.filter(event => event.seriesId === id)
    expect(series.map(event => event.start)).toEqual([
      '2099-03-01T09:00:00.000Z',
      '2099-03-08T09:00:00.000Z',
      '2099-03-15T09:00:00.000Z',
    ])
    expect(series.every(event => event.isRecurring && !event.isException)).toBe(true)
  })

  it('rejects incomplete, invalid, and excessive ranges', async () => {
    expect((await call(Events, 'GET', req({ token: auth(), query: { start: START } }))).error).toMatch(/both/)
    expect((await call(Events, 'GET', req({ token: auth(), query: { start: 'bad', end: END } }))).error).toMatch(/range/)
    expect((await call(Events, 'GET', req({ token: auth(), query: {
      start: '2090-01-01T00:00:00.000Z', end: '2092-01-01T00:00:00.000Z',
    } }))).error).toMatch(/400 days/)
  })
})

describe('GET /calendar/events/:id', () => {
  it('404s without an id', async () => {
    expect((await call(Event, 'GET', req({ token: auth() }))).notFound).toBe(true)
  })

  it('404s for an unknown id', async () => {
    expect((await call(Event, 'GET', req({ token: auth(), paths: { id: 'ghost' } }))).notFound).toBe(true)
  })

  it('403s for another user event', async () => {
    const id = await create()
    expect((await call(Event, 'GET', req({ token: other(), paths: { id } }))).forbidden).toBe(true)
  })
})

describe('writable provider calendars', () => {
  const providerEvent = {
    id: 'provider-writable-event', userEmail: 'alice@remote.example', title: 'Provider meeting', start: START, end: END,
    allDay: false, location: '', description: '', attendees: [], organizer: 'alice@remote.example', source: 'provider', readOnly: false,
    icsUid: 'remote-uid', recurrence: '', recurrenceDates: [], exceptionDates: [], timeZone: 'UTC', seriesId: '', recurrenceId: '',
    accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', accountLabel: 'Work', accountColor: '#336699', calendarId: 'calendar_work',
    calendarName: 'Work calendar', calendarColor: '#112233', providerResourceId: 'caldav_resource', createdAt: START, updatedAt: START,
  }

  it('lists writable calendars and routes create, update, get, and delete through the provider boundary', async () => {
    const calls = []
    app.providerCalendars = {
      available: () => true,
      calendars: async boundaryToken => { calls.push(['calendars', boundaryToken]); return [{ accountId: providerEvent.accountId, calendarId: providerEvent.calendarId, calendarName: providerEvent.calendarName, writable: true }] },
      find: async (boundaryToken, id) => { calls.push(['find', boundaryToken, id]); return id === providerEvent.id ? { event: providerEvent, resource: {} } : null },
      create: async (boundaryToken, calendarId, event, key) => { calls.push(['create', boundaryToken, calendarId, event, key]); return { id: 'calmut_create', resourceId: 'caldav_created', state: 'applied' } },
      update: async (boundaryToken, id, event, recurrenceId, key) => { calls.push(['update', boundaryToken, id, event, recurrenceId, key]); return { id: 'calmut_update', state: 'applied' } },
      delete: async (boundaryToken, id, recurrenceId, key) => { calls.push(['delete', boundaryToken, id, recurrenceId, key]); return { id: 'calmut_delete', state: 'applied' } },
      list: async () => [providerEvent],
    }
    try {
      const callerToken = auth()
      const listed = await call(Calendars, 'GET', req({ token: callerToken }))
      expect(listed.calendars[0]).toMatchObject({ calendarId: 'calendar_work', writable: true })

      const created = await call(Events, 'POST', req({ token: callerToken, body: { title: 'Remote created', start: START, end: END, calendarId: 'calendar_work', idempotencyKey: 'create-request' } }))
      expect(created).toEqual({ saved: true, id: 'caldav_created' })

      const loaded = await call(Event, 'GET', req({ token: callerToken, paths: { id: providerEvent.id } }))
      expect(loaded).toMatchObject({ id: providerEvent.id, source: 'provider', readOnly: false })

      const updated = await call(Event, 'PUT', req({ token: callerToken, paths: { id: providerEvent.id }, body: { title: 'Provider renamed', idempotencyKey: 'update-request' } }))
      expect(updated).toMatchObject({ updated: true, scope: 'event', event: { title: 'Provider renamed', source: 'provider' } })

      const deleted = await call(Event, 'DELETE', req({ token: callerToken, paths: { id: providerEvent.id }, query: { idempotencyKey: 'delete-request' } }))
      expect(deleted).toEqual({ deleted: true, scope: 'event', remainingSeries: false })

      expect(calls.find(call => call[0] === 'create').slice(2)).toMatchObject(['calendar_work', { title: 'Remote created' }, 'create-request'])
      expect(calls.find(call => call[0] === 'update').slice(2)).toMatchObject([providerEvent.id, { title: 'Provider renamed' }, '', 'update-request'])
      expect(calls.find(call => call[0] === 'delete').slice(2)).toEqual([providerEvent.id, '', 'delete-request'])
      expect(calls.every(call => call[1] === callerToken)).toBe(true)
    } finally {
      app.providerCalendars = { available: () => false }
    }
  })
})

describe('PUT /calendar/events/:id', () => {
  it('404s without an id', async () => {
    expect((await call(Event, 'PUT', req({ token: auth(), body: {} }))).notFound).toBe(true)
  })

  it('404s for an unknown id', async () => {
    expect((await call(Event, 'PUT', req({ token: auth(), paths: { id: 'ghost' }, body: {} }))).notFound).toBe(true)
  })

  it('403s for another user event', async () => {
    const id = await create()
    expect((await call(Event, 'PUT', req({ token: other(), paths: { id }, body: {} }))).forbidden).toBe(true)
  })

  it('400s on an unparseable start or end', async () => {
    const id = await create()
    expect((await call(Event, 'PUT', req({ token: auth(), paths: { id }, body: { start: 'nope' } }))).error).toMatch(/start/)
    expect((await call(Event, 'PUT', req({ token: auth(), paths: { id }, body: { end: 'nope' } }))).error).toMatch(/end/)
  })

  it('400s when the patched end precedes the start', async () => {
    const id = await create()
    const res = await call(Event, 'PUT', req({ token: auth(), paths: { id }, body: { end: '2020-01-01T00:00:00.000Z' } }))
    expect(res.error).toMatch(/on or after/)
  })

  it('applies patchable fields and ignores the rest', async () => {
    const id = await create()
    const res = await call(Event, 'PUT', req({ token: auth(), paths: { id }, body: { title: 'Renamed', userEmail: 'hacker@x.com', source: 'ics' } }))
    expect(res.updated).toBe(true)
    const after = await call(Event, 'GET', req({ token: auth(), paths: { id } }))
    expect(after.title).toBe('Renamed')
    expect(after.source).toBe('manual')
  })

  it('converts between timed and all-day event representations atomically', async () => {
    const id = await create()
    const allDay = await call(Event, 'PUT', req({
      token: auth(), paths: { id }, body: { allDay: true, start: '2099-03-08', end: '2099-03-10' },
    }))
    expect(allDay.event).toMatchObject({ allDay: true, start: '2099-03-08', end: '2099-03-10' })

    const rejectedMixed = await call(Event, 'PUT', req({
      token: auth(), paths: { id }, body: { allDay: false, start: '2099-03-08', end: '2099-03-10' },
    }))
    expect(rejectedMixed.error).toMatch(/date-time/)

    const timed = await call(Event, 'PUT', req({
      token: auth(), paths: { id }, body: { allDay: false, start: START, end: END },
    }))
    expect(timed.event).toMatchObject({ allDay: false, start: START, end: END })
  })

  it('edits and deletes one date-only recurrence occurrence by date identity', async () => {
    const id = await create({
      allDay: true, start: '2099-03-01', end: '2099-03-02', recurrence: 'FREQ=DAILY;COUNT=3',
    })
    const changed = await call(Event, 'PUT', req({
      token: auth(), paths: { id }, query: { scope: 'instance', occurrenceStart: '2099-03-02' },
      body: { start: '2099-03-05', end: '2099-03-06', allDay: true },
    }))
    expect(changed.event).toMatchObject({
      allDay: true, recurrenceId: '2099-03-02', start: '2099-03-05', end: '2099-03-06',
    })
    const deleted = await call(Event, 'DELETE', req({
      token: auth(), paths: { id }, query: { scope: 'instance', occurrenceStart: '2099-03-03' },
    }))
    expect(deleted).toEqual({ deleted: true, scope: 'instance', remainingSeries: true })
  })

  it('edits one future recurrence without changing the series', async () => {
    const id = await create({ recurrence: 'FREQ=WEEKLY;COUNT=3', timeZone: 'UTC' })
    const occurrenceStart = '2099-03-08T09:00:00.000Z'
    const res = await call(Event, 'PUT', req({
      token: auth(), paths: { id },
      query: { scope: 'instance', occurrenceStart },
      body: { title: 'Moved occurrence', start: '2099-03-09T11:00:00.000Z', end: '2099-03-09T12:00:00.000Z' },
    }))
    expect(res).toMatchObject({ updated: true, scope: 'instance' })
    expect(res.event).toMatchObject({ title: 'Moved occurrence', seriesId: id, recurrenceId: occurrenceStart, isException: true })

    const listed = await call(Events, 'GET', req({ token: auth(), query: {
      start: '2099-03-01T00:00:00.000Z', end: '2099-03-31T23:59:59.999Z',
    } }))
    const series = listed.filter(event => event.seriesId === id)
    expect(series).toHaveLength(3)
    expect(series.some(event => event.start === occurrenceStart)).toBe(false)
    expect(series.some(event => event.title === 'Moved occurrence' && event.isException)).toBe(true)
  })

  it('applies occurrence-relative move and resize edits to the whole timed series', async () => {
    const id = await create({ recurrence: 'FREQ=WEEKLY;COUNT=3', timeZone: 'UTC' })
    const occurrenceStart = '2099-03-08T09:00:00.000Z'
    const res = await call(Event, 'PUT', req({
      token: auth(), paths: { id }, query: { scope: 'series', occurrenceStart },
      body: { start: '2099-03-08T11:00:00.000Z', end: '2099-03-08T14:00:00.000Z' },
    }))
    expect(res).toMatchObject({ updated: true, scope: 'series' })
    expect(res.event).toMatchObject({
      start: '2099-03-01T11:00:00.000Z', end: '2099-03-01T14:00:00.000Z',
    })

    const listed = await call(Events, 'GET', req({ token: auth(), query: {
      start: '2099-03-01T00:00:00.000Z', end: '2099-03-31T23:59:59.999Z',
    } }))
    const series = listed.filter(event => event.seriesId === id)
    expect(series).toHaveLength(3)
    expect(series.every(event => Date.parse(event.end) - Date.parse(event.start) === 3 * 60 * 60_000)).toBe(true)
  })

  it('applies occurrence-relative date movement and span changes to an all-day series', async () => {
    const id = await create({
      allDay: true, start: '2099-03-01', end: '2099-03-02', recurrence: 'FREQ=DAILY;COUNT=3',
    })
    const res = await call(Event, 'PUT', req({
      token: auth(), paths: { id }, query: { scope: 'series', occurrenceStart: '2099-03-02' },
      body: { allDay: true, start: '2099-03-04', end: '2099-03-07' },
    }))
    expect(res.event).toMatchObject({ start: '2099-03-03', end: '2099-03-06', allDay: true })
  })

  it('rejects invalid recurrence rules, invited-event edits, and past-event edits', async () => {
    const id = await create()
    expect((await call(Event, 'PUT', req({ token: auth(), paths: { id }, body: { recurrence: 'FREQ=HOURLY' } }))).error).toMatch(/frequency/)

    const [docId, stored] = await app.calendarEvents.findById(id)
    await app.calendarEvents.delete(docId)
    await app.calendarEvents.put({ ...stored, source: 'invite', organizer: 'boss@other.com' })
    expect((await call(Event, 'PUT', req({ token: auth(), paths: { id }, body: { title: 'Nope' } }))).forbidden).toBe(true)

    const pastId = await create({ start: '2020-01-01T09:00:00.000Z', end: '2020-01-01T10:00:00.000Z' })
    expect((await call(Event, 'PUT', req({ token: auth(), paths: { id: pastId }, body: { title: 'Nope' } }))).conflict).toBe(true)
  })
})

describe('DELETE /calendar/events/:id', () => {
  it('404s without an id', async () => {
    expect((await call(Event, 'DELETE', req({ token: auth() }))).notFound).toBe(true)
  })

  it('404s for an unknown id', async () => {
    expect((await call(Event, 'DELETE', req({ token: auth(), paths: { id: 'ghost' } }))).notFound).toBe(true)
  })

  it('403s for another user event', async () => {
    const id = await create()
    expect((await call(Event, 'DELETE', req({ token: other(), paths: { id } }))).forbidden).toBe(true)
  })

  it('deletes the event', async () => {
    const id = await create()
    expect((await call(Event, 'DELETE', req({ token: auth(), paths: { id } }))).deleted).toBe(true)
    expect((await call(Event, 'GET', req({ token: auth(), paths: { id } }))).notFound).toBe(true)
  })

  it('deletes one occurrence with an explicit remaining-series disclosure', async () => {
    const id = await create({ recurrence: 'FREQ=WEEKLY;COUNT=3', timeZone: 'UTC' })
    const occurrenceStart = '2099-03-08T09:00:00.000Z'
    const deleted = await call(Event, 'DELETE', req({ token: auth(), paths: { id }, query: { scope: 'instance', occurrenceStart } }))
    expect(deleted).toEqual({ deleted: true, scope: 'instance', remainingSeries: true })
    const listed = await call(Events, 'GET', req({ token: auth(), query: {
      start: '2099-03-01T00:00:00.000Z', end: '2099-03-31T23:59:59.999Z',
    } }))
    expect(listed.filter(event => event.seriesId === id)).toHaveLength(2)
  })

  it('deletes a complete series and its exceptions', async () => {
    const id = await create({ recurrence: 'FREQ=WEEKLY;COUNT=3', timeZone: 'UTC' })
    const occurrenceStart = '2099-03-08T09:00:00.000Z'
    await call(Event, 'PUT', req({ token: auth(), paths: { id }, query: { scope: 'instance', occurrenceStart }, body: { title: 'One edit' } }))
    expect(await call(Event, 'DELETE', req({ token: auth(), paths: { id }, query: { scope: 'series' } })))
      .toEqual({ deleted: true, scope: 'series', remainingSeries: false })
    expect((await app.calendarEvents.list('alice@example.com')).some(event => event.id === id || event.seriesId === id)).toBe(false)
  })
})

describe('POST /calendar/invites/respond', () => {
  it('400s without an eventId', async () => {
    expect((await call(Respond, 'POST', req({ token: auth(), body: {} }))).error).toMatch(/eventId/)
  })

  it('400s on an unrecognised status', async () => {
    const res = await call(Respond, 'POST', req({ token: auth(), body: { eventId: 'x', status: 'maybe' } }))
    expect(res.error).toMatch(/accepted, declined, or tentative/)
  })

  it('404s for an unknown event', async () => {
    const res = await call(Respond, 'POST', req({ token: auth(), body: { eventId: 'ghost', status: 'accepted' } }))
    expect(res.notFound).toBe(true)
  })

  it('404s for another user event', async () => {
    const id = await create()
    const res = await call(Respond, 'POST', req({ token: other(), body: { eventId: id, status: 'accepted' } }))
    expect(res.notFound).toBe(true)
  })

  it('adds the caller as an attendee when absent', async () => {
    const id = await create()
    const res = await call(Respond, 'POST', req({ token: auth(), body: { eventId: id, status: 'tentative' } }))
    expect(res).toEqual({ responded: true, status: 'tentative', replySent: false })
    const after = await call(Event, 'GET', req({ token: auth(), paths: { id } }))
    expect(after.attendees).toEqual([{ email: 'alice@example.com', status: 'tentative' }])
  })

  it('updates the caller existing attendee row', async () => {
    const id = await create({ attendees: [{ email: 'alice@example.com', name: 'Alice' }] })
    await call(Respond, 'POST', req({ token: auth(), body: { eventId: id, status: 'declined' } }))
    const after = await call(Event, 'GET', req({ token: auth(), paths: { id } }))
    expect(after.attendees[0]).toEqual({ email: 'alice@example.com', name: 'Alice', status: 'declined' })
  })

  it('sends an iCalendar reply when the invite has a uid and organizer', async () => {
    const id = await create({ organizer: 'boss@other.com' })
    const [docId, event] = await app.calendarEvents.findById(id)
    await app.calendarEvents.delete(docId)
    await app.calendarEvents.put({ ...event, icsUid: 'uid-1@other.com' })

    const res = await call(Respond, 'POST', req({ token: auth(), body: { eventId: id, status: 'accepted' } }))
    expect(res.replySent).toBe(true)
  })
})

describe('GET /calendar/team-events', () => {
  it('requires auth', async () => {
    expect((await call(Team, 'GET', req())).unauthorized).toBe(true)
  })

  it('returns nothing for a caller with no domains', async () => {
    expect(await call(Team, 'GET', req({ token: token({ email: 'viewer@example.com' }), query: {} }))).toEqual([])
  })

  it('excludes private calendars and the caller own events', async () => {
    await app.users.put({ email: 'private@example.com', aliases: [], domains: ['example.com'], role: 'viewer' })
    const res = await call(Team, 'GET', req({ token: auth(), query: {} }))
    expect(res.every(e => e.ownerEmail !== 'alice@example.com')).toBe(true)
    expect(res.every(e => e.ownerEmail !== 'private@example.com')).toBe(true)
  })

  it('redacts busy calendars and exposes full ones', async () => {
    await app.users.put({ email: 'busy@example.com', aliases: [], domains: ['example.com'], role: 'viewer', calendarVisibility: 'busy' })
    await app.users.put({ email: 'open@example.com', aliases: [], domains: ['example.com'], role: 'viewer', calendarVisibility: 'full', name: 'Open' })
    await app.calendarEvents.put({ id: 'busy-1', userEmail: 'busy@example.com', title: 'Secret', location: 'HQ', start: START, end: END, createdAt: START, updatedAt: START })
    await app.calendarEvents.put({ id: 'open-1', userEmail: 'open@example.com', title: 'Public', location: 'HQ', start: START, end: END, createdAt: START, updatedAt: START })

    const res = await call(Team, 'GET', req({ token: auth(), query: {} }))
    const busy = res.find(e => e.ownerEmail === 'busy@example.com')
    const open = res.find(e => e.ownerEmail === 'open@example.com')
    expect(busy.title).toBe('Busy')
    expect(busy.location).toBe('')
    expect(open.title).toBe('Public')
    expect(open.location).toBe('HQ')
    expect(open.ownerName).toBe('Open')
  })

  it('excludes users from other domains', async () => {
    await app.users.put({ email: 'far@other.com', aliases: [], domains: ['other.com'], role: 'viewer', calendarVisibility: 'full' })
    await app.calendarEvents.put({ id: 'far-1', userEmail: 'far@other.com', title: 'Far', start: START, end: END, createdAt: START, updatedAt: START })
    const res = await call(Team, 'GET', req({ token: auth(), query: {} }))
    expect(res.every(e => e.ownerEmail !== 'far@other.com')).toBe(true)
  })
})
