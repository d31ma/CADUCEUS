// @ts-check

const DATE_PATTERN = /^(\d{4})-(\d{2})-(\d{2})$/
const DAY_MS = 24 * 60 * 60 * 1000

/** @param {unknown} value */
export function isCalendarDate(value) {
  if (typeof value !== 'string') return false
  const match = value.match(DATE_PATTERN)
  if (!match) return false
  const instant = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])))
  return instant.toISOString().slice(0, 10) === value
}

/** @param {string} value @param {number} days */
export function addCalendarDays(value, days) {
  if (!isCalendarDate(value) || !Number.isInteger(days)) throw new TypeError('calendar date and integer day count are required')
  return new Date(dateEpoch(value) + days * DAY_MS).toISOString().slice(0, 10)
}

/** @param {string} start @param {string} end */
export function calendarDaysBetween(start, end) {
  if (!isCalendarDate(start) || !isCalendarDate(end)) throw new TypeError('two calendar dates are required')
  return Math.round((dateEpoch(end) - dateEpoch(start)) / DAY_MS)
}

/** @param {string} date */
export function allDayRangeForDate(date) {
  if (!isCalendarDate(date)) throw new TypeError('calendar date is required')
  return { start: date, end: addCalendarDays(date, 1), allDay: true }
}

/** @param {{start:string,end:string}} range @param {string} targetDate */
export function shiftAllDayRangeToDate(range, targetDate) {
  const start = calendarDateValue(range.start)
  const end = calendarDateValue(range.end)
  if (!start || !end || !isCalendarDate(targetDate)) throw new TypeError('valid all-day range and target date are required')
  const duration = Math.max(1, calendarDaysBetween(start, end))
  return { start: targetDate, end: addCalendarDays(targetDate, duration), allDay: true }
}

/** @param {{start:string,end:string}} range @param {string} targetDate */
export function moveTimedRangeToDate(range, targetDate) {
  if (!isCalendarDate(targetDate)) throw new TypeError('target date is required')
  const start = new Date(range.start)
  const end = new Date(range.end || range.start)
  if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime())) throw new TypeError('valid timed range is required')
  const target = parseLocalDate(targetDate)
  const nextStart = new Date(start)
  nextStart.setFullYear(target.getFullYear(), target.getMonth(), target.getDate())
  const nextEnd = new Date(nextStart.getTime() + Math.max(0, end.getTime() - start.getTime()))
  return { start: nextStart.toISOString(), end: nextEnd.toISOString(), allDay: false }
}

/** Convert an exclusive all-day range into editable local wall-clock values. */
export function timedRangeForAllDayDates(range) {
  const startDate = calendarDateValue(range.start)
  const endDate = calendarDateValue(range.end)
  if (!startDate || !endDate || calendarDaysBetween(startDate, endDate) < 1) {
    throw new TypeError('valid all-day range is required')
  }
  const start = parseLocalDate(startDate)
  start.setHours(9, 0, 0, 0)
  const end = parseLocalDate(addCalendarDays(endDate, -1))
  end.setHours(10, 0, 0, 0)
  return { start: localDateTimeValue(start), end: localDateTimeValue(end), allDay: false }
}

/** @param {Record<string, any>} event @param {string} date */
export function eventCoversCalendarDate(event, date) {
  if (!isCalendarDate(date)) return false
  if (event?.allDay) {
    const start = calendarDateValue(event.start)
    const end = calendarDateValue(event.end || event.start)
    return !!start && !!end && start <= date && date < end
  }
  const start = new Date(event?.start)
  return Number.isFinite(start.getTime()) && localIsoDate(start) === date
}

/** Resolve the stable cursor instant used to reveal an event in another view. */
export function calendarCursorForEvent(event) {
  if (event?.allDay) {
    const date = calendarDateValue(event.start)
    return date ? parseLocalDate(date) : null
  }
  const instant = new Date(event?.start)
  return Number.isFinite(instant.getTime()) ? instant : null
}

/** @param {unknown} value */
export function calendarDateValue(value) {
  if (isCalendarDate(value)) return /** @type {string} */ (value)
  if (typeof value !== 'string' || !Number.isFinite(Date.parse(value))) return ''
  // Legacy all-day rows were persisted at UTC midnight. Reading their UTC
  // date preserves the civil day the writer intended during migration.
  return value.slice(0, 10)
}

/** @param {Date} date */
export function localIsoDate(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/** @param {Date} date */
export function localDateTimeValue(date) {
  const pad = value => String(value).padStart(2, '0')
  return `${localIsoDate(date)}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** @param {string} value */
export function parseLocalDate(value) {
  if (!isCalendarDate(value)) throw new TypeError('calendar date is required')
  const [year, month, day] = value.split('-').map(Number)
  return new Date(year, month - 1, day, 12, 0, 0, 0)
}

/** @param {string} value */
function dateEpoch(value) {
  const [year, month, day] = value.split('-').map(Number)
  return Date.UTC(year, month - 1, day)
}
