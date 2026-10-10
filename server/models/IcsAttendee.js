/**
 * @typedef {object} IcsAttendee
 * @property {string} email
 * @property {string} [name]
 * @property {'accepted'|'declined'|'tentative'|'needsAction'|'delegated'|'completed'} [status]
 *
 * @typedef {object} IcsEvent
 * @property {string} uid
 * @property {string} summary
 * @property {string} start
 * @property {string} end
 * @property {boolean} allDay
 * @property {string} [location]
 * @property {string} [description]
 * @property {string} [organizer]
 * @property {IcsAttendee[]} attendees
 * @property {string} [rrule]
 * @property {string[]} [recurrenceDates]
 * @property {string[]} [exceptionDates]
 * @property {string} [recurrenceId]
 * @property {'THISANDFUTURE'} [recurrenceRange]
 * @property {string} [timeZone]
 * @property {number} [sequence]
 * @property {string} [duration]
 * @property {'confirmed'|'cancelled'|'tentative'} [status]
 * @property {string} [dtstamp]
 *
 * @typedef {object} IcsCalendar
 * @property {'REQUEST'|'CANCEL'|'REPLY'|'PUBLISH'|'COUNTER'|'REFRESH'|'ADD'|'DECLINECOUNTER'|null} method
 * @property {string} prodid
 * @property {IcsEvent[]} events
 */
export {}
