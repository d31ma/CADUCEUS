import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { Ics } from '../features/calendar/services/Ics.js'

@RequestRoute("/calendar/invites/respond")
@SignedIn
export class CalendarInvitesRespondController {
  static VALID_STATUSES = /** @type {const} */ (['accepted', 'declined', 'tentative'])

  /**
 * POST /calendar/invites/respond
 *
 * Apply the caller's RSVP to an inbound invite. Updates the attendee's
 * status on the stored event and sends an iCalendar METHOD:REPLY back to the
 * organizer (best-effort — a failed reply doesn't roll back the local
 * status update; the user has already made a decision).
 *
 * @param {object} params
 * @param {{ eventId: string, status: 'accepted' | 'declined' | 'tentative' }} params.body
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @PostRoute
  async post(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ body, auth }) => {
    const eventId = typeof body?.eventId === 'string' ? body.eventId : ''
    const status = typeof body?.status === 'string' ? body.status : ''
    if (!eventId) return Respond.r400('eventId required')
    if (!CalendarInvitesRespondController.VALID_STATUSES.includes(/** @type {any} */ (status))) {
      return Respond.r400('status must be accepted, declined, or tentative')
    }

    const app = await getApp()
    const [docId, event] = await app.calendarEvents.findById(eventId)
    if (!event || !docId) return Respond.r404('Event not found')
    if (event.userEmail !== auth.email) return Respond.r404('Event not found')
    if (event.status === 'cancelled') return Respond.r409('Cancelled invitations cannot be answered')

    const attendees = Array.isArray(event.attendees) ? [...event.attendees] : []
    const me = auth.email.toLowerCase()
    const idx = attendees.findIndex(a => (a.email || '').toLowerCase() === me)
    if (idx >= 0) attendees[idx] = { ...attendees[idx], status: /** @type {any} */ (status) }
    else attendees.push({ email: me, status: /** @type {any} */ (status) })

    // Fylo's putData always inserts a new docId; update in place by deleting
    // the prior version first.
    await app.calendarEvents.delete(docId)
    await app.calendarEvents.put({
      ...event,
      attendees,
      updatedAt: new Date().toISOString(),
    })

    let replySent = false
    if (event.icsUid && event.organizer) {
      try {
        const ics = Ics.buildIcsReply({
          uid: event.icsUid,
          organizer: event.organizer,
          attendee: auth.email,
          status: /** @type {any} */ (status),
          summary: event.title,
          start: event.start,
          end: event.end,
          allDay: !!event.allDay,
        })
        const subject = `${status === 'accepted' ? 'Accepted' : status === 'declined' ? 'Declined' : 'Tentative'}: ${event.title}`
        await app.smtp.sendEmail(auth.email, {
          to: [event.organizer],
          subject,
          text: `${auth.email} responded "${status}" to "${event.title}".`,
          attachments: [{
            filename: 'invite.ics',
            contentBase64: Buffer.from(ics, 'utf-8').toString('base64'),
            contentType: 'text/calendar; method=REPLY; charset=utf-8',
          }],
        })
        replySent = true
      } catch (err) {
        console.warn('[calendar] failed to send RSVP reply', { eventId, err: String(err) })
      }
    }

    return { responded: true, status, replySent }
    })
  }
}
