import { randomUUID } from 'node:crypto'
import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { CalendarRecurrence } from '../features/calendar/services/CalendarRecurrence.js'
import { CalendarDate } from '../features/calendar/services/CalendarDate.js'
import { toCalendarEventView } from '../features/calendar/services/event-view.js'
import { mergeCalendarEvents } from '../features/calendar/services/ProviderCalendarService.js'
import { accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'

@RequestRoute("/calendar/events")
@SignedIn
export class CalendarEventsController {
  static ATTENDEE_STATUSES = ['accepted', 'declined', 'tentative', 'needsAction', 'delegated', 'completed']

  /**
 * POST /calendar/events
 *
 * Create a new calendar event on the caller's calendar.
 *
 * @param {object} params
 * @param {any} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, context, auth }) => {
    if ('allDay' in (body || {}) && typeof body.allDay !== 'boolean') return Respond.r400('allDay must be a boolean')
    const fields = CalendarEventsController._validateTimes(body)
    if (fields instanceof Error) return Respond.r400(fields.message)

    const timeZone = typeof body?.timeZone === 'string' ? body.timeZone : 'UTC'
    if (!CalendarRecurrence.isTimeZone(timeZone)) return Respond.r400('timeZone must be an IANA timezone')
    const recurrence = CalendarRecurrence.validate(body?.recurrence, { start: fields.start, timeZone })
    if (!recurrence.ok) return Respond.r400(recurrence.error)

    const app = await getApp()
    const idempotencyKey = CalendarEventsController._idempotencyKey(body?.idempotencyKey, 'create')
    const event = CalendarEventsController._buildEvent({ ...body, idempotencyKey, recurrence: recurrence.rule, timeZone }, fields, auth.email)
    if (typeof body?.calendarId === 'string' && body.calendarId.trim()) {
      if (!app.providerCalendars) return Respond.r503('Provider calendar service unavailable')
      try {
        const mutation = await app.providerCalendars.create(bearerToken(context), body.calendarId.trim(), event, idempotencyKey)
        return { saved: true, id: mutation.resourceId || mutation.id }
      } catch (error) {
        return CalendarEventsController._providerFailure(error)
      }
    }
    const [, replay] = await app.calendarEvents.findByIdempotencyKey(auth.email, idempotencyKey)
    if (replay) return { saved: true, id: replay.id }
    await app.calendarEvents.put(event)
    return { saved: true, id: event.id }
    })
  }

  /** @param {unknown} value @param {string} operation */
  static _idempotencyKey(value, operation) {
    const candidate = typeof value === 'string' ? value.trim() : ''
    return candidate && candidate.length <= 128 ? candidate : `calendar-${operation}-${randomUUID()}`
  }

  /** @param {unknown} error */
  static _providerFailure(error) {
    const message = error instanceof Error ? error.message : ''
    if (/changed|conflict/i.test(message)) return Respond.r409(message || 'Provider calendar changed')
    if (/not found/i.test(message)) return Respond.r404(message)
    return accountFailure(error)
  }

  /**
   * Trim and validate the three required fields.
   * @param {any} body
   * @returns {{ title: string, start: string, end: string } | Error}
   */
  static _validateTimes(body) {
    const title = typeof body?.title === 'string' ? body.title.trim() : ''
    if (!title) return new Error('title is required')
    const range = CalendarDate.validateRange(body?.start, body?.end, body?.allDay === true)
    if (range instanceof Error) return range
    return { title, ...range }
  }

  /**
   * Drop entries without an email and normalise the rest.
   * @param {unknown} input
   * @returns {import('../features/calendar/repositories/CalendarEventsRepository.js').Attendee[]}
   */
  static _normalizeAttendees(input) {
    if (!Array.isArray(input)) return []
    return input
      .filter(/** @param {any} a */ a => a && typeof a.email === 'string')
      .map(/** @param {any} a */ a => ({
        email: a.email.toLowerCase(),
        name: typeof a.name === 'string' ? a.name : '',
        status: /** @type {any} */ (CalendarEventsController.ATTENDEE_STATUSES.includes(a.status) ? a.status : 'needsAction'),
      }))
  }

  /**
   * @param {any} body
   * @param {{ title: string, start: string, end: string }} fields
   * @param {string} userEmail
   * @returns {import('../features/calendar/repositories/CalendarEventsRepository.js').CalendarEvent}
   */
  static _buildEvent(body, fields, userEmail) {
    const now = new Date().toISOString()
    return {
      id: `evt-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
      userEmail,
      ...fields,
      allDay: !!body.allDay,
      location: typeof body.location === 'string' ? body.location.trim() : '',
      description: typeof body.description === 'string' ? body.description : '',
      attendees: CalendarEventsController._normalizeAttendees(body.attendees),
      organizer: typeof body.organizer === 'string' ? body.organizer.toLowerCase() : userEmail,
      source: 'manual',
      idempotencyKey: typeof body.idempotencyKey === 'string' ? body.idempotencyKey : undefined,
      recurrence: typeof body.recurrence === 'string' ? body.recurrence : '',
      recurrenceDates: [],
      exceptionDates: [],
      timeZone: typeof body.timeZone === 'string' ? body.timeZone : 'UTC',
      reminderMinutes: typeof body.reminderMinutes === 'number' ? body.reminderMinutes : undefined,
      status: 'confirmed',
      createdAt: now,
      updatedAt: now,
    }
  }

  /**
 * GET /calendar/events?start=<ISO>&end=<ISO>
 *
 * Return calendar events owned by the caller, optionally clipped to an
 * inclusive time window. An event "overlaps" the window if its `[start, end]`
 * interval intersects `[query.start, query.end]`.
 *
 * @param {object} params
 * @param {{ bearer?: { token: string }, query?: Record<string, string> }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, auth }) => {
    const start = context.query?.start || ''
    const end = context.query?.end || ''
    if ((start && !end) || (!start && end)) return Respond.r400('calendar range requires both start and end')
    const app = await getApp()
    const localEvents = await app.calendarEvents.list(auth.email, {
      start: start || undefined,
      end: end || undefined,
    })
    let events = localEvents
    if (app.providerCalendars) {
      try {
        const providerEvents = await app.providerCalendars.list(bearerToken(context))
        events = mergeCalendarEvents(localEvents, providerEvents)
      } catch (error) {
        return accountFailure(error)
      }
    }

    /** @type {Array<Record<string, any>>} */
    let visible = events
    if (start && end) {
      try { visible = CalendarRecurrence.expand(events, { start, end }) }
      catch (error) { return Respond.r400(error instanceof Error ? error.message : 'calendar range is invalid') }
    }
    return visible
      .sort((a, b) => Date.parse(a.start) - Date.parse(b.start))
      .map(event => toCalendarEventView({
        ...event,
        readOnly: event.readOnly === true ||
          (event.source === 'invite' && event.organizer && String(event.organizer).toLowerCase() !== auth.email.toLowerCase()) ||
          CalendarDate.isPast(event),
      }))
    })
  }
}
