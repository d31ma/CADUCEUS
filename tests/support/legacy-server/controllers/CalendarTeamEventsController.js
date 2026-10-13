import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { CalendarRecurrence } from '../shared/calendar/CalendarRecurrence.js'

@RequestRoute("/calendar/team-events")
@SignedIn
export class CalendarTeamEventsController {
  /** @type {CalendarEventsHelper} */
  @Needs
  calendarEventsHelper

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /**
 * GET /calendar/team-events?start=&end=
 *
 * Return calendar events from every same-domain user who has opted into
 * sharing (`calendarVisibility !== 'private'`), redacted per their setting:
 *
 *   - `busy` — title/location/description/attendees stripped; only the time
 *              range and ownerEmail/ownerName are returned.
 *   - `full` — full event payload with ownerEmail/ownerName attached.
 *
 * The caller's own events are excluded; those come from `/calendar/events`.
 *
 * @param {object} params
 * @param {{ bearer?: { token: string }, query?: Record<string, string> }} params.context
 * @returns {Promise<unknown>}
 */
  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ context, auth }) => {
    const viewerDomains = (auth.domains || []).map(d => d.toLowerCase())
    if (viewerDomains.length === 0) return []

    const app = this
    const allUsers = await app.usersHelper.list()
    const sharers = allUsers.filter(u => {
      if ((u.email || '').toLowerCase() === (auth.email || '').toLowerCase()) return false
      const vis = /** @type {any} */ (u).calendarVisibility || 'private'
      if (vis === 'private') return false
      return (u.domains || []).map(d => d.toLowerCase()).some(d => viewerDomains.includes(d))
    })

    const start = context.query?.start || undefined
    const end = context.query?.end || undefined
    if ((start && !end) || (!start && end)) return Respond.r400('calendar range requires both start and end')

    /** @type {Array<Record<string, unknown>>} */
    const out = []
    // Sequential per-owner queries are fine for tens of teammates; if this
    // ever needs to scale further it becomes a single multi-user Fylo query.
    for (const owner of sharers) {
      const stored = await app.calendarEventsHelper.list(owner.email, { start, end })
      /** @type {Array<Record<string, any>>} */
      let events = stored
      if (start && end) {
        try { events = CalendarRecurrence.expand(stored, { start, end }) }
        catch (error) { return Respond.r400(error instanceof Error ? error.message : 'calendar range is invalid') }
      }
      const vis = /** @type {any} */ (owner).calendarVisibility
      for (const ev of events) {
        const base = {
          id: `${owner.email}::${ev.id}`,
          ownerEmail: owner.email,
          ownerName: /** @type {any} */ (owner).name || '',
          ownerVisibility: vis,
          start: ev.start,
          end: ev.end,
          allDay: !!ev.allDay,
          seriesId: ev.seriesId || '',
          recurrenceId: ev.recurrenceId || '',
          isRecurring: !!ev.isRecurring || !!ev.recurrence || !!ev.seriesId,
          isException: typeof ev.isException === 'boolean' ? ev.isException : !!ev.seriesId,
          readOnly: true,
        }
        if (vis === 'full') {
          out.push({
            ...base,
            title: ev.title,
            location: ev.location || '',
            description: ev.description || '',
            attendees: ev.attendees || [],
            source: 'team',
            originalSource: ev.source || 'manual',
            status: ev.status || 'confirmed',
          })
        } else {
          out.push({
            ...base,
            title: 'Busy',
            location: '',
            description: '',
            attendees: [],
            source: 'team',
            status: 'confirmed',
          })
        }
      }
    }

    return out.sort((a, b) => Date.parse(/** @type {string} */ (a.start)) - Date.parse(/** @type {string} */ (b.start)))
    })
  }
}
