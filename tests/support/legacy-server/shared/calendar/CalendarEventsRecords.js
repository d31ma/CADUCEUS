import { Fylo } from '../persistence/Fylo.js'
import { CalendarDate } from './CalendarDate.js'

export class CalendarEventsRecords {
  /**
   * @param {Record<string, unknown>} raw
   * @returns {CalendarEvent & { docId?: string }}
   */
  static hydrate(raw) {
    /** @type {any} */
    const e = { ...raw }
    if (typeof e.attendeesJson === 'string') {
      try { e.attendees = JSON.parse(e.attendeesJson) } catch { e.attendees = [] }
      delete e.attendeesJson
    } else if (!Array.isArray(e.attendees)) {
      e.attendees = []
    }
    return e
  }
}
