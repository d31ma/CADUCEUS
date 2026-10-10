// @ts-check

import { rrulestr } from '../../deps/npm/rrule.mjs'
import { CalendarDate } from './CalendarDate.js'

const MAX_RULE_BYTES = 1024
const MAX_COUNT = 10_000
const MAX_INTERVAL = 365
const MAX_RANGE_DAYS = 400
const MAX_OCCURRENCES = 2_000
const RANGE_PADDING_MS = 2 * 24 * 60 * 60 * 1000
const DAY_MS = 24 * 60 * 60 * 1000

const ALLOWED_FREQUENCIES = new Set(['DAILY', 'WEEKLY', 'MONTHLY', 'YEARLY'])
const ALLOWED_KEYS = new Set([
  'FREQ', 'INTERVAL', 'COUNT', 'UNTIL', 'WKST', 'BYDAY', 'BYMONTHDAY',
  'BYYEARDAY', 'BYWEEKNO', 'BYMONTH', 'BYSETPOS',
])
const WEEKDAYS = '(?:MO|TU|WE|TH|FR|SA|SU)'

/**
 * Expands persisted RFC 5545 recurrence rules into bounded calendar instances.
 *
 * The recurrence library deliberately deals in "floating" UTC dates. This
 * service translates each generated wall-clock value through the event's IANA
 * timezone so a 09:00 meeting remains at 09:00 across daylight-saving changes.
 * Stable `seriesId` + `recurrenceId` identities allow one generated occurrence
 * to be replaced by an edited or cancelled exception document.
 */
export class CalendarRecurrence {
  /** @param {unknown} value */
  static isTimeZone(value) { return validTimeZone(value) }

  /**
   * Convert an RFC 5545 DATE or DATE-TIME into one absolute ISO timestamp.
   * Zoned wall times use the same DST disambiguation as recurrence expansion.
   * @param {string} raw
   * @param {string} [timeZone]
   */
  static parseDateTime(raw, timeZone = 'UTC') {
    const value = String(raw || '').trim()
    if (/^\d{8}$/.test(value)) return CalendarDate.fromCompact(value)
    const match = value.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})(Z?)$/)
    if (match) {
      const [, year, month, day, hour, minute, second, utc] = match
      const floating = new Date(Date.UTC(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), Number(second)))
      return (utc ? floating : fromFloating(floating, validTimeZone(timeZone) ? timeZone : 'UTC')).toISOString()
    }
    const parsed = Date.parse(value)
    return Number.isFinite(parsed) ? new Date(parsed).toISOString() : value
  }

  /**
   * Validate and canonicalize one RRULE value. Empty means non-recurring.
   * Expensive sub-daily expansion is intentionally unsupported.
   *
   * @param {unknown} input
   * @param {{ start?: string, timeZone?: string }} [event]
   * @returns {{ ok: true, rule: string } | { ok: false, error: string }}
   */
  static validate(input, event = {}) {
    if (input == null || String(input).trim() === '') return { ok: true, rule: '' }
    let value = String(input).trim()
    if (/^RRULE:/i.test(value)) value = value.slice(6)
    if (new TextEncoder().encode(value).byteLength > MAX_RULE_BYTES || /[\r\n]/.test(value)) {
      return invalid('recurrence rule is too long or contains multiple lines')
    }
    if (event.start && !CalendarDate.isDate(event.start) && !CalendarDate.isDateTime(event.start)) return invalid('event start is invalid')
    if (!validTimeZone(event.timeZone || 'UTC')) return invalid('timeZone must be an IANA timezone')

    /** @type {Map<string, string>} */
    const parts = new Map()
    for (const rawPart of value.split(';')) {
      const separator = rawPart.indexOf('=')
      if (separator <= 0 || separator === rawPart.length - 1) return invalid('recurrence rule contains an invalid part')
      const key = rawPart.slice(0, separator).trim().toUpperCase()
      const partValue = rawPart.slice(separator + 1).trim().toUpperCase()
      if (!ALLOWED_KEYS.has(key)) return invalid(`recurrence rule part ${key} is unsupported`)
      if (parts.has(key)) return invalid(`recurrence rule part ${key} is duplicated`)
      parts.set(key, partValue)
    }

    const frequency = parts.get('FREQ') || ''
    if (!ALLOWED_FREQUENCIES.has(frequency)) return invalid('recurrence frequency must be DAILY, WEEKLY, MONTHLY, or YEARLY')
    if (!boundedInteger(parts.get('INTERVAL'), 1, MAX_INTERVAL)) return invalid(`recurrence interval must be from 1 through ${MAX_INTERVAL}`)
    if (!boundedInteger(parts.get('COUNT'), 1, MAX_COUNT)) return invalid(`recurrence count must be from 1 through ${MAX_COUNT}`)
    if (parts.has('COUNT') && parts.has('UNTIL')) return invalid('recurrence cannot contain both COUNT and UNTIL')
    if (parts.has('UNTIL') && !/^\d{8}(?:T\d{6}Z?)?$/.test(parts.get('UNTIL') || '')) return invalid('recurrence UNTIL is invalid')
    if (parts.has('WKST') && !new RegExp(`^${WEEKDAYS}$`).test(parts.get('WKST') || '')) return invalid('recurrence WKST is invalid')
    if (parts.has('BYDAY') && !listMatches(parts.get('BYDAY'), new RegExp(`^[+-]?(?:[1-9]|[1-4]\d|5[0-3])?${WEEKDAYS}$`))) return invalid('recurrence BYDAY is invalid')
    if (parts.has('BYMONTHDAY') && !integerList(parts.get('BYMONTHDAY'), -31, 31, false)) return invalid('recurrence BYMONTHDAY is invalid')
    if (parts.has('BYYEARDAY') && !integerList(parts.get('BYYEARDAY'), -366, 366, false)) return invalid('recurrence BYYEARDAY is invalid')
    if (parts.has('BYWEEKNO') && !integerList(parts.get('BYWEEKNO'), -53, 53, false)) return invalid('recurrence BYWEEKNO is invalid')
    if (parts.has('BYMONTH') && !integerList(parts.get('BYMONTH'), 1, 12, true)) return invalid('recurrence BYMONTH is invalid')
    if (parts.has('BYSETPOS') && !integerList(parts.get('BYSETPOS'), -366, 366, false)) return invalid('recurrence BYSETPOS is invalid')

    const ordered = ['FREQ', 'INTERVAL', 'COUNT', 'UNTIL', 'WKST', 'BYDAY', 'BYMONTHDAY', 'BYYEARDAY', 'BYWEEKNO', 'BYMONTH', 'BYSETPOS']
    const rule = ordered.filter(key => parts.has(key)).map(key => `${key}=${parts.get(key)}`).join(';')
    try {
      rrulestr(rule, { dtstart: recurrenceStart(event.start || '2000-01-01T00:00:00.000Z') })
    } catch {
      return invalid('recurrence rule is invalid')
    }
    return { ok: true, rule }
  }

  /**
   * @param {Array<Record<string, any>>} events
   * @param {{ start: string, end: string }} range
   * @returns {Array<Record<string, any>>}
   */
  static expand(events, range) {
    const windowStart = Date.parse(range?.start)
    const windowEnd = Date.parse(range?.end)
    if (!Number.isFinite(windowStart) || !Number.isFinite(windowEnd) || windowEnd < windowStart) {
      throw new RangeError('calendar range is invalid')
    }
    if (windowEnd - windowStart > MAX_RANGE_DAYS * DAY_MS) {
      throw new RangeError(`calendar range cannot exceed ${MAX_RANGE_DAYS} days`)
    }

    const masters = events.filter(event => !event.seriesId)
    const exceptionsBySeries = groupExceptions(events.filter(event => event.seriesId))
    /** @type {Array<Record<string, any>>} */
    const output = []
    const knownSeries = new Set(masters.map(event => event.id))

    for (const master of masters) {
      if (master.status === 'cancelled') continue
      const validation = CalendarRecurrence.validate(master.recurrence, master)
      if (!validation.ok || !validation.rule) {
        if (master.status !== 'cancelled' && overlaps(master, windowStart, windowEnd)) output.push(asOrdinary(master))
        continue
      }
      const instances = expandMaster(master, validation.rule, range)
      const exceptions = exceptionsBySeries.get(master.id) || new Map()
      for (const instance of instances) {
        const replacement = exceptions.get(instance.recurrenceId)
        if (replacement) {
          if (replacement.status !== 'cancelled' && overlaps(replacement, windowStart, windowEnd)) {
            output.push(asException(replacement, master.id))
          }
        } else if (overlaps(instance, windowStart, windowEnd)) {
          output.push(instance)
        }
        if (output.length > MAX_OCCURRENCES) throw new RangeError(`calendar range exceeds ${MAX_OCCURRENCES} occurrences`)
      }
    }

    // An orphaned exception must remain visible rather than silently losing a
    // user's edit. It is still marked as an exception and remains owner-scoped.
    for (const candidate of events.filter(event => event.seriesId && !knownSeries.has(event.seriesId))) {
      if (candidate.status !== 'cancelled' && overlaps(candidate, windowStart, windowEnd)) {
        output.push(asException(candidate, candidate.seriesId))
      }
    }

    return output.sort((left, right) => Date.parse(left.start) - Date.parse(right.start) || String(left.id).localeCompare(String(right.id)))
  }
}

/** @param {Record<string, any>} master @param {string} rule @param {{start:string,end:string}} range */
function expandMaster(master, rule, range) {
  if (master.allDay && CalendarDate.isDate(master.start) && CalendarDate.isDate(master.end)) {
    return expandAllDayMaster(master, rule, range)
  }
  const zone = validTimeZone(master.timeZone) ? master.timeZone : 'UTC'
  const sourceStart = new Date(master.start)
  const duration = Math.max(0, Date.parse(master.end || master.start) - sourceStart.getTime())
  const floatingStart = toFloating(sourceStart, zone)
  const recurrence = rrulestr(rule, { dtstart: floatingStart })
  const paddedStart = new Date(Date.parse(range.start) - RANGE_PADDING_MS)
  const paddedEnd = new Date(Date.parse(range.end) + RANGE_PADDING_MS)
  const floatingValues = recurrence.between(paddedStart, paddedEnd, true)
  /** @type {Map<string, Date>} */
  const actualValues = new Map()
  for (const floating of floatingValues) {
    const actual = fromFloating(floating, zone)
    actualValues.set(actual.toISOString(), actual)
  }
  for (const value of normalizeDateList(master.recurrenceDates, false)) {
    const actual = new Date(value)
    actualValues.set(actual.toISOString(), actual)
  }
  const exclusions = new Set(normalizeDateList(master.exceptionDates, false).map(value => new Date(value).toISOString()))

  return [...actualValues.values()]
    .filter(actual => !exclusions.has(actual.toISOString()))
    .sort((left, right) => left.getTime() - right.getTime())
    .map(actual => {
      const recurrenceId = actual.toISOString()
      return {
        ...master,
        id: occurrenceIdentity(master.id, recurrenceId),
        seriesId: master.id,
        recurrenceId,
        start: recurrenceId,
        end: new Date(actual.getTime() + duration).toISOString(),
        recurrence: rule,
        isRecurring: true,
        isException: false,
      }
    })
}

/** @param {Record<string, any>} master @param {string} rule @param {{start:string,end:string}} range */
function expandAllDayMaster(master, rule, range) {
  const durationDays = Math.max(1, CalendarDate.daysBetween(master.start, master.end))
  const recurrence = rrulestr(rule, { dtstart: recurrenceStart(master.start) })
  const paddedStart = new Date(Date.parse(range.start) - RANGE_PADDING_MS)
  const paddedEnd = new Date(Date.parse(range.end) + RANGE_PADDING_MS)
  const dates = new Set(
    recurrence.between(paddedStart, paddedEnd, true).map(value => value.toISOString().slice(0, 10)),
  )
  for (const value of normalizeDateList(master.recurrenceDates, true)) dates.add(value)
  const exclusions = new Set(normalizeDateList(master.exceptionDates, true))
  return [...dates]
    .filter(value => !exclusions.has(value))
    .sort()
    .map(recurrenceId => ({
      ...master,
      id: occurrenceIdentity(master.id, recurrenceId),
      seriesId: master.id,
      recurrenceId,
      start: recurrenceId,
      end: CalendarDate.addDays(recurrenceId, durationDays),
      allDay: true,
      recurrence: rule,
      isRecurring: true,
      isException: false,
    }))
}

/** @param {Array<Record<string, any>>} exceptions */
function groupExceptions(exceptions) {
  /** @type {Map<string, Map<string, Record<string, any>>>} */
  const grouped = new Map()
  for (const exception of exceptions) {
    const dateIdentity = CalendarDate.coerce(exception.recurrenceId)
    const instantIdentity = CalendarDate.normalizeOccurrence(exception.recurrenceId, false)
    if (!exception.seriesId || (!dateIdentity && !instantIdentity)) continue
    if (!grouped.has(exception.seriesId)) grouped.set(exception.seriesId, new Map())
    // Index both identity forms when possible. This keeps a series occurrence
    // attached to its master if an explicit edit changes only its all-day/timed
    // representation, and bridges legacy UTC-midnight all-day exceptions.
    if (dateIdentity) grouped.get(exception.seriesId)?.set(dateIdentity, exception)
    if (instantIdentity) grouped.get(exception.seriesId)?.set(instantIdentity, exception)
  }
  return grouped
}

/** @param {Record<string, any>} event */
function asOrdinary(event) {
  return { ...event, isRecurring: false, isException: false }
}

/** @param {Record<string, any>} event @param {string} seriesId */
function asException(event, seriesId) {
  return { ...event, seriesId, isRecurring: true, isException: true }
}

/** @param {Record<string, any>} event @param {number} start @param {number} end */
function overlaps(event, start, end) {
  return CalendarDate.overlaps(event, start, end)
}

/** @param {string} seriesId @param {string} recurrenceId */
function occurrenceIdentity(seriesId, recurrenceId) {
  return `${seriesId}::${encodeURIComponent(recurrenceId)}`
}

/** @param {Date} instant @param {string} timeZone */
function toFloating(instant, timeZone) {
  const parts = zonedParts(instant, timeZone)
  return new Date(Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second, instant.getUTCMilliseconds()))
}

/**
 * Convert an rrule floating wall-clock value into a real instant. Exact matches
 * cover ordinary and repeated clock times; nonexistent spring-forward times use
 * Temporal-compatible "later" disambiguation so the event remains visible.
 *
 * @param {Date} floating
 * @param {string} timeZone
 */
function fromFloating(floating, timeZone) {
  if (timeZone === 'UTC') return new Date(floating)
  const desired = floating.getTime()
  const offsets = new Set([
    offsetAt(new Date(desired - 36 * 60 * 60 * 1000), timeZone),
    offsetAt(new Date(desired), timeZone),
    offsetAt(new Date(desired + 36 * 60 * 60 * 1000), timeZone),
  ])
  /** @type {Array<{instant:number, wall:number}>} */
  const candidates = []
  for (const offset of offsets) {
    const instant = desired - offset
    const wall = wallEpoch(new Date(instant), timeZone)
    candidates.push({ instant, wall })
  }
  const exact = candidates.filter(candidate => candidate.wall === desired).sort((a, b) => a.instant - b.instant)
  if (exact.length) return new Date(exact[0].instant)
  const later = candidates.filter(candidate => candidate.wall > desired).sort((a, b) => a.wall - b.wall || a.instant - b.instant)
  if (later.length) return new Date(later[0].instant)
  return new Date(candidates.sort((a, b) => Math.abs(a.wall - desired) - Math.abs(b.wall - desired))[0].instant)
}

/** @param {Date} instant @param {string} timeZone */
function offsetAt(instant, timeZone) {
  return wallEpoch(instant, timeZone) - instant.getTime() + instant.getUTCMilliseconds()
}

/** @param {Date} instant @param {string} timeZone */
function wallEpoch(instant, timeZone) {
  const parts = zonedParts(instant, timeZone)
  return Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour, parts.minute, parts.second, instant.getUTCMilliseconds())
}

/** @param {Date} instant @param {string} timeZone */
function zonedParts(instant, timeZone) {
  const formatter = new Intl.DateTimeFormat('en-CA', {
    timeZone, year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
  })
  /** @type {Record<string, number>} */
  const parts = {}
  for (const part of formatter.formatToParts(instant)) {
    if (part.type !== 'literal') parts[part.type] = Number(part.value)
  }
  return /** @type {{year:number,month:number,day:number,hour:number,minute:number,second:number}} */ (parts)
}

/** @param {unknown} input */
function normalizeDateList(input, allDay) {
  if (!Array.isArray(input)) return []
  if (allDay) return input.slice(0, 256).map(value => CalendarDate.coerce(value)).filter(Boolean)
  return input.slice(0, 256).filter(value => typeof value === 'string' && CalendarDate.isDateTime(value))
}

/** @param {string} value */
function recurrenceStart(value) {
  return new Date(CalendarDate.isDate(value) ? `${value}T00:00:00.000Z` : value)
}

/** @param {string | undefined} value @param {number} min @param {number} max */
function boundedInteger(value, min, max) {
  if (value == null) return true
  if (!/^\d+$/.test(value)) return false
  const number = Number(value)
  return number >= min && number <= max
}

/** @param {string | undefined} value @param {RegExp} pattern */
function listMatches(value, pattern) {
  return !!value && value.split(',').length <= 64 && value.split(',').every(part => pattern.test(part))
}

/** @param {string | undefined} value @param {number} min @param {number} max @param {boolean} allowZero */
function integerList(value, min, max, allowZero) {
  if (!value) return false
  const values = value.split(',')
  if (values.length > 64) return false
  return values.every(part => /^[-+]?\d+$/.test(part) && Number(part) >= min && Number(part) <= max && (allowZero || Number(part) !== 0))
}

/** @param {unknown} value */
function validTimeZone(value) {
  if (typeof value !== 'string' || value.length > 100) return false
  try { new Intl.DateTimeFormat('en', { timeZone: value }).format(); return true } catch { return false }
}

/** @param {string} error */
function invalid(error) {
  return /** @type {const} */ ({ ok: false, error })
}
