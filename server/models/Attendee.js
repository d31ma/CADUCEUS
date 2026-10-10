/**
 * @typedef {object} Attendee
 * @property {string} email
 * @property {string} [name]
 * @property {'accepted'|'declined'|'tentative'|'needsAction'|'delegated'|'completed'} [status]
 *
 * @typedef {object} CalendarEvent
 * @property {string} id
 * @property {string} userEmail
 * @property {string} title
 * @property {string} start
 * @property {string} end
 * @property {boolean} [allDay]
 * @property {string} [location]
 * @property {string} [description]
 * @property {Attendee[]} [attendees]
 * @property {string} [organizer]
 * @property {'manual'|'invite'} source
 * @property {string} [sourceEmailId]
 * @property {string[]} [sourceEmailIds]
 * @property {string} [icsUid]
 * @property {string} [recurrence]
 * @property {string[]} [recurrenceDates]
 * @property {string[]} [exceptionDates]
 * @property {string} [timeZone]
 * @property {string} [seriesId]
 * @property {string} [recurrenceId]
 * @property {number} [reminderMinutes]
 * @property {'confirmed'|'cancelled'|'tentative'} [status]
 * @property {string} createdAt
 * @property {string} updatedAt
 * @property {string} [idempotencyKey]
 */
export {}
