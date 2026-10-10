/**
 * Calendar-date operations for RFC 5545 DATE values.
 *
 * A DATE is a civil day, not an instant. Keeping it as `YYYY-MM-DD` prevents
 * timezone conversion and DST transitions from moving an all-day event onto a
 * neighboring day. All-day `end` values are exclusive.
 */
export class CalendarDate {
  // @ts-check

  static #DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/

  static #DAY_MS = 24 * 60 * 60 * 1000

  /** @param {string} value */
  static #dateEpoch(value) {
    const match = /** @type {RegExpMatchArray} */ (value.match(CalendarDate.#DATE_PATTERN))
    return Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3]))
  }

  /** @param {Date} value */
  static #formatUTCDate(value) {
    return value.toISOString().slice(0, 10)
  }

  /** @param {unknown} value */
  static isDate(value) {
    if (typeof value !== 'string') return false
    const match = value.match(CalendarDate.#DATE_PATTERN)
    if (!match) return false
    const [, year, month, day] = match
    const instant = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day)))
    return instant.getUTCFullYear() === Number(year) &&
      instant.getUTCMonth() + 1 === Number(month) &&
      instant.getUTCDate() === Number(day)
  }

  /** @param {unknown} value */
  static isDateTime(value) {
    return typeof value === 'string' && value.includes('T') && Number.isFinite(Date.parse(value))
  }

  /** @param {string} value @param {number} days */
  static addDays(value, days) {
    if (!CalendarDate.isDate(value) || !Number.isInteger(days)) throw new TypeError('calendar date and integer day count are required')
    return CalendarDate.#formatUTCDate(new Date(CalendarDate.#dateEpoch(value) + days * CalendarDate.#DAY_MS))
  }

  /** @param {string} start @param {string} end */
  static daysBetween(start, end) {
    if (!CalendarDate.isDate(start) || !CalendarDate.isDate(end)) throw new TypeError('two calendar dates are required')
    return Math.round((CalendarDate.#dateEpoch(end) - CalendarDate.#dateEpoch(start)) / CalendarDate.#DAY_MS)
  }

  /**
   * @param {unknown} rawStart
   * @param {unknown} rawEnd
   * @param {boolean} allDay
   * @returns {{start:string,end:string}|Error}
   */
  static validateRange(rawStart, rawEnd, allDay) {
    const start = typeof rawStart === 'string' ? rawStart.trim() : ''
    const end = typeof rawEnd === 'string' ? rawEnd.trim() : ''
    if (allDay) {
      if (!CalendarDate.isDate(start) || !CalendarDate.isDate(end)) {
        return new Error('all-day start and end must be date-only YYYY-MM-DD values')
      }
      if (end <= start) return new Error('all-day end must be after start')
      return { start, end }
    }
    if (!CalendarDate.isDateTime(start)) return new Error('valid start (ISO 8601 date-time) is required')
    if (!CalendarDate.isDateTime(end)) return new Error('valid end (ISO 8601 date-time) is required')
    if (Date.parse(end) < Date.parse(start)) return new Error('end must be on or after start')
    return { start, end }
  }

  /** @param {string} value */
  static fromCompact(value) {
    return /^\d{8}$/.test(value)
      ? `${value.slice(0, 4)}-${value.slice(4, 6)}-${value.slice(6, 8)}`
      : value
  }

  /** @param {unknown} value @returns {string} */
  static coerce(value) {
    if (CalendarDate.isDate(value)) return String(value)
    return typeof value === 'string' && CalendarDate.isDateTime(value) ? value.slice(0, 10) : ''
  }

  /** @param {unknown} value @param {boolean} allDay @returns {string} */
  static normalizeOccurrence(value, allDay) {
    if (allDay) return CalendarDate.isDate(value) ? String(value) : ''
    return CalendarDate.isDateTime(value) ? new Date(/** @type {string} */ (value)).toISOString() : ''
  }

  /** @param {Record<string, any>} event @param {number} rangeStart @param {number} rangeEnd */
  static overlaps(event, rangeStart, rangeEnd) {
    if (event.allDay && CalendarDate.isDate(event.start) && CalendarDate.isDate(event.end)) {
      const start = CalendarDate.#dateEpoch(event.start)
      const exclusiveEnd = CalendarDate.#dateEpoch(event.end)
      return exclusiveEnd > rangeStart && start <= rangeEnd
    }
    const start = Date.parse(event.start)
    const end = Date.parse(event.end || event.start)
    return Number.isFinite(start) && Number.isFinite(end) && end >= rangeStart && start <= rangeEnd
  }

  /** @param {Record<string, any>} event @param {number} [now] */
  static isPast(event, now = Date.now()) {
    if (event.allDay && CalendarDate.isDate(event.end)) {
      return event.end <= CalendarDate.todayInTimeZone(event.timeZone || 'UTC', now)
    }
    const end = Date.parse(event.end || event.start)
    return Number.isFinite(end) && end < now
  }

  /** @param {string} timeZone @param {number} [now] */
  static todayInTimeZone(timeZone, now = Date.now()) {
    let formatter
    try {
      formatter = new Intl.DateTimeFormat('en-CA', {
        timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
      })
    } catch {
      // Legacy records may predate timezone validation. Keep read paths
      // available and deterministic until those rows are rewritten.
      formatter = new Intl.DateTimeFormat('en-CA', {
        timeZone: 'UTC', year: 'numeric', month: '2-digit', day: '2-digit',
      })
    }
    /** @type {Record<string, string>} */
    const parts = {}
    for (const part of formatter.formatToParts(new Date(now))) {
      if (part.type !== 'literal') parts[part.type] = part.value
    }
    return `${parts.year}-${parts.month}-${parts.day}`
  }
}
