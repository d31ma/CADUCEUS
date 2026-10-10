import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { CalendarRecurrence } from '../shared/calendar/CalendarRecurrence.js'
import { CalendarDate } from '../shared/calendar/CalendarDate.js'
import { toCalendarEventView } from '../shared/calendar/event-view.js'
import { accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'
import { CalendarEventsIdController as EventsHandler } from './CalendarEventsController.js'

@RequestRoute("/calendar/events/{id}")
@SignedIn
export class CalendarEventsIdController {
  /** Fields the client is allowed to patch — identity + ownership are immutable. */
  static PATCHABLE_KEYS = new Set([
    'title', 'start', 'end', 'allDay', 'location', 'description',
    'attendees', 'organizer', 'recurrence', 'reminderMinutes', 'status', 'timeZone',
  ])

  /**
 * DELETE /calendar/events/:id
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, context, auth }) => {
    const id = paths?.id
    if (!id) return Respond.r404('Event not found')

    const app = await getApp()
    const local = await app.calendarEvents.findById(id)
    /** @type {string | null} */
    let docId = local[0]
    /** @type {Record<string, any> | null} */
    let event = local[1]
    const providerCalendars = app.providerCalendars
    let provider = false
    if ((!event || !docId) && providerCalendars) {
      try {
        const found = await providerCalendars.find(bearerToken(context), id)
        if (found) { event = found.event; docId = ''; provider = true }
      } catch (error) { return accountFailure(error) }
    }
    if (!event || (!docId && !provider)) return Respond.r404('Event not found')
    if (!provider && event.userEmail !== auth.email) return Respond.r403('Access denied')

    const scope = context?.query?.scope || 'series'
    if (!['series', 'instance'].includes(scope)) return Respond.r400('scope must be series or instance')
    const mutationError = CalendarEventsIdController._mutationError(event, auth.email, context?.query?.occurrenceStart || '')
    if (mutationError) return mutationError

    if (provider) {
      if (!providerCalendars) return Respond.r503('Provider calendar service unavailable')
      const recurrenceId = scope === 'instance' ? CalendarEventsIdController._normalizeOccurrenceStart(context?.query?.occurrenceStart, !!event.allDay) : ''
      if (scope === 'instance' && (!event.recurrence || !recurrenceId || !CalendarEventsIdController._resolveOccurrence(event, recurrenceId))) {
        return Respond.r400('instance scope requires a valid recurring occurrence')
      }
      try {
        await providerCalendars.delete(bearerToken(context), event.id, recurrenceId, EventsHandler.idempotencyKey(context?.query?.idempotencyKey, 'delete'))
        return { deleted: true, scope: recurrenceId ? 'instance' : event.recurrence ? 'series' : 'event', remainingSeries: !!recurrenceId }
      } catch (error) { return EventsHandler.providerFailure(error) }
    }

    if (scope === 'instance') {
      if (!event.recurrence) return Respond.r400('instance scope requires a recurring series')
      const occurrenceStart = CalendarEventsIdController._normalizeOccurrenceStart(context?.query?.occurrenceStart, !!event.allDay)
      if (!occurrenceStart) return Respond.r400('occurrenceStart must identify a valid series occurrence')
      const occurrence = CalendarEventsIdController._resolveOccurrence(event, occurrenceStart)
      if (!occurrence) return Respond.r404('Occurrence not found')
      const [existingDocId, existing] = await app.calendarEvents.findException(event.id, occurrenceStart)
      if (existingDocId) await app.calendarEvents.delete(existingDocId)
      await app.calendarEvents.put(/** @type {any} */ ({
        ...(existing || occurrence),
        id: existing?.id || CalendarEventsIdController._newExceptionId(),
        userEmail: event.userEmail,
        seriesId: event.id,
        recurrenceId: occurrenceStart,
        recurrence: '',
        recurrenceDates: [],
        exceptionDates: [],
        status: 'cancelled',
        createdAt: existing?.createdAt || new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      }))
      return { deleted: true, scope: 'instance', remainingSeries: true }
    }

    if (event.recurrence) await app.calendarEvents.deleteSeries(event.id, auth.email)
    else if (docId) await app.calendarEvents.delete(docId)
    else return Respond.r404('Event not found')
    return { deleted: true, scope: event.recurrence ? 'series' : 'event', remainingSeries: false }
    })
  }

  /**
 * PUT /calendar/events/:id
 *
 * Partial update of an event. Body is merged into the stored event; fields
 * outside {@link PATCHABLE_KEYS} are ignored. Only the owner may edit.
 *
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {Record<string, unknown>} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, context, auth }) => {
    const id = paths?.id
    if (!id) return Respond.r404('Event not found')

    const app = await getApp()
    const local = await app.calendarEvents.findById(id)
    /** @type {string | null} */
    let docId = local[0]
    /** @type {Record<string, any> | null} */
    let event = local[1]
    const providerCalendars = app.providerCalendars
    let provider = false
    if ((!event || !docId) && providerCalendars) {
      try {
        const found = await providerCalendars.find(bearerToken(context), id)
        if (found) { event = found.event; docId = ''; provider = true }
      } catch (error) { return accountFailure(error) }
    }
    if (!event || (!docId && !provider)) return Respond.r404('Event not found')
    if (!provider && event.userEmail !== auth.email) return Respond.r403('Access denied')

    const scope = context?.query?.scope || 'series'
    if (!['series', 'instance'].includes(scope)) return Respond.r400('scope must be series or instance')
    const mutationError = CalendarEventsIdController._mutationError(event, auth.email, context?.query?.occurrenceStart || '')
    if (mutationError) return mutationError

    /** @type {Record<string, unknown>} */
    const patch = {}
    for (const [key, value] of Object.entries(body || {})) {
      if (CalendarEventsIdController.PATCHABLE_KEYS.has(key)) patch[key] = value
    }

    const nextStart = typeof patch.start === 'string' ? patch.start : event.start
    const nextEnd = typeof patch.end === 'string' ? patch.end : event.end
    if ('allDay' in patch && typeof patch.allDay !== 'boolean') return Respond.r400('allDay must be a boolean')
    const nextAllDay = typeof patch.allDay === 'boolean' ? patch.allDay : !!event.allDay
    let range = CalendarDate.validateRange(nextStart, nextEnd, nextAllDay)
    if (range instanceof Error) return Respond.r400(range.message)
    const hasOccurrenceRelativeTime = 'start' in patch || 'end' in patch
    if (scope === 'series' && event.recurrence && context?.query?.occurrenceStart && hasOccurrenceRelativeTime) {
      if (!('start' in patch) || !('end' in patch)) {
        return Respond.r400('occurrence-relative series edits require both start and end')
      }
      const occurrenceStart = CalendarEventsIdController._normalizeOccurrenceStart(context.query.occurrenceStart, !!event.allDay)
      if (!occurrenceStart) return Respond.r400('occurrenceStart must identify a valid series occurrence')
      const occurrence = CalendarEventsIdController._resolveOccurrence(event, occurrenceStart)
      if (!occurrence) return Respond.r404('Occurrence not found')
      if (nextAllDay !== !!event.allDay) {
        return Respond.r400('change the series all-day representation separately from an occurrence-relative resize')
      }
      range = CalendarEventsIdController._masterRangeFromOccurrenceEdit(event, occurrence, range, nextAllDay)
    }
    patch.start = range.start
    patch.end = range.end
    if (typeof patch.title === 'string') {
      patch.title = patch.title.trim()
      if (!patch.title) return Respond.r400('title is required')
    }
    const nextTimeZone = typeof patch.timeZone === 'string' ? patch.timeZone : (event.timeZone || 'UTC')
    if (!CalendarRecurrence.isTimeZone(nextTimeZone)) return Respond.r400('timeZone must be an IANA timezone')
    if ('recurrence' in patch) {
      if (scope === 'instance') return Respond.r400('one occurrence cannot define a recurrence rule')
      const recurrence = CalendarRecurrence.validate(patch.recurrence, { start: range.start, timeZone: nextTimeZone })
      if (!recurrence.ok) return Respond.r400(recurrence.error)
      patch.recurrence = recurrence.rule
    }

    if (scope === 'instance') {
      if (!event.recurrence) return Respond.r400('instance scope requires a recurring series')
      const occurrenceStart = CalendarEventsIdController._normalizeOccurrenceStart(context?.query?.occurrenceStart, !!event.allDay)
      if (!occurrenceStart) return Respond.r400('occurrenceStart must identify a valid series occurrence')
      const occurrence = CalendarEventsIdController._resolveOccurrence(event, occurrenceStart)
      if (!occurrence) return Respond.r404('Occurrence not found')
      const [existingDocId, existing] = provider ? [null, null] : await app.calendarEvents.findException(event.id, occurrenceStart)
      /** @type {import('../features/calendar/repositories/CalendarEventsRepository.js').CalendarEvent} */
      const exception = /** @type {any} */ ({
        ...(existing || occurrence),
        ...patch,
        id: existing?.id || CalendarEventsIdController._newExceptionId(),
        userEmail: event.userEmail,
        source: event.source,
        seriesId: event.id,
        recurrenceId: occurrenceStart,
        recurrence: '',
        recurrenceDates: [],
        exceptionDates: [],
        status: 'confirmed',
        createdAt: existing?.createdAt || new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      })
      if (provider) {
        if (!providerCalendars) return Respond.r503('Provider calendar service unavailable')
        try {
          await providerCalendars.update(bearerToken(context), event.id, exception, occurrenceStart, EventsHandler.idempotencyKey(body?.idempotencyKey, 'update-instance'))
        } catch (error) { return EventsHandler.providerFailure(error) }
      } else {
        if (existingDocId) await app.calendarEvents.delete(existingDocId)
        await app.calendarEvents.put(exception)
      }
      return { updated: true, scope: 'instance', event: toCalendarEventView({ ...exception, isRecurring: true, isException: true }) }
    }

    /** @type {import('../features/calendar/repositories/CalendarEventsRepository.js').CalendarEvent} */
    const next = /** @type {any} */ ({
      ...event,
      ...patch,
      id: event.id,
      userEmail: event.userEmail,
      source: event.source,
      createdAt: event.createdAt,
      updatedAt: new Date().toISOString(),
    })
    if (provider) {
      if (!providerCalendars) return Respond.r503('Provider calendar service unavailable')
      try {
        await providerCalendars.update(bearerToken(context), event.id, next, '', EventsHandler.idempotencyKey(body?.idempotencyKey, 'update'))
      } catch (error) { return EventsHandler.providerFailure(error) }
    } else {
      // Fylo's putData always inserts under a fresh docId — to update in place
      // we delete the prior version first, then write the patched one.
      if (!docId) return Respond.r404('Event not found')
      await app.calendarEvents.delete(docId)
      await app.calendarEvents.put(next)
    }
    return { updated: true, scope: event.recurrence ? 'series' : 'event', event: toCalendarEventView(next) }
    })
  }

  /**
 * GET /calendar/events/:id
 * @param {object} params
 * @param {{ id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, context, auth }) => {
    const id = paths?.id
    if (!id) return Respond.r404('Event not found')

    const app = await getApp()
    const local = await app.calendarEvents.findById(id)
    /** @type {Record<string, any> | null} */
    let event = local[1]
    let provider = false
    if (!event && app.providerCalendars) {
      try {
        const found = await app.providerCalendars.find(bearerToken(context), id)
        if (found) { event = found.event; provider = true }
      } catch (error) { return accountFailure(error) }
    }
    if (!event) return Respond.r404('Event not found')
    if (!provider && event.userEmail !== auth.email) return Respond.r403('Access denied')

    return toCalendarEventView({
      ...event,
        readOnly: (event.source === 'invite' && event.organizer && String(event.organizer).toLowerCase() !== auth.email.toLowerCase()) ||
        (!event.recurrence && CalendarDate.isPast(event)),
    })
    })
  }

  /** @param {unknown} value */
  static _normalizeOccurrenceStart(value, allDay = false) {
    return CalendarDate.normalizeOccurrence(value, allDay)
  }

  /** @param {Record<string, any>} series @param {string} recurrenceId */
  static _resolveOccurrence(series, recurrenceId) {
    const instant = Date.parse(CalendarDate.isDate(recurrenceId) ? `${recurrenceId}T00:00:00.000Z` : recurrenceId)
    const occurrences = CalendarRecurrence.expand([series], {
      start: new Date(instant - 2 * 24 * 60 * 60 * 1000).toISOString(),
      end: new Date(instant + 2 * 24 * 60 * 60 * 1000).toISOString(),
    })
    return occurrences.find(candidate => candidate.recurrenceId === recurrenceId) || null
  }

  /**
   * Apply an edited occurrence's move delta and duration to its recurring
   * master. The occurrence date itself must never become the new master date.
   * @param {Record<string, any>} master
   * @param {Record<string, any>} occurrence
   * @param {{start:string,end:string}} edited
   * @param {boolean} allDay
   */
  static _masterRangeFromOccurrenceEdit(master, occurrence, edited, allDay) {
    if (allDay) {
      const movedDays = CalendarDate.daysBetween(occurrence.start, edited.start)
      const durationDays = CalendarDate.daysBetween(edited.start, edited.end)
      const start = CalendarDate.addDays(master.start, movedDays)
      return { start, end: CalendarDate.addDays(start, durationDays) }
    }
    const moveMs = Date.parse(edited.start) - Date.parse(occurrence.start)
    const durationMs = Date.parse(edited.end) - Date.parse(edited.start)
    const startMs = Date.parse(master.start) + moveMs
    return {
      start: new Date(startMs).toISOString(),
      end: new Date(startMs + durationMs).toISOString(),
    }
  }

  /** @param {Record<string, any>} event @param {string} owner @param {unknown} occurrenceStart */
  static _mutationError(event, owner, occurrenceStart) {
    if (event.source === 'invite' && event.organizer && String(event.organizer).toLowerCase() !== owner.toLowerCase()) {
      return Respond.r403('Invited events are read-only; respond from the invitation instead')
    }
    if (occurrenceStart) {
      if (event.allDay && CalendarDate.isDate(occurrenceStart)) {
        if (String(occurrenceStart) < CalendarDate.todayInTimeZone(event.timeZone || 'UTC')) return Respond.r409('Past event occurrences are read-only')
      } else {
        const instant = Date.parse(String(occurrenceStart))
        if (!Number.isFinite(instant)) return Respond.r400('occurrenceStart must be an ISO 8601 date-time string')
        if (instant < Date.now()) return Respond.r409('Past event occurrences are read-only')
      }
    } else if (!event.recurrence && CalendarDate.isPast(event)) {
      return Respond.r409('Past events are read-only')
    }
    return null
  }

  static _newExceptionId() {
    return `evt-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
  }
}
