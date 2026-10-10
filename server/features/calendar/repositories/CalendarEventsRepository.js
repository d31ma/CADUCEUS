import { Collections, collect } from '../../../platform/persistence/Fylo.js'
import { CalendarDate } from '../services/CalendarDate.js'

export class CalendarEventsRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * Store / upsert a calendar event.
   *
   * Fylo refuses to index an array of objects, and `attendees` is exactly that.
   * We serialize it (along with anything else Fylo can't index) into a single
   * `attendeesJson` string at write-time and re-hydrate on read.
   *
   * @param {CalendarEvent} event
   * @returns {Promise<string>}
   */
  async put(event) {
    /** @type {Record<string, unknown>} */
    const stored = { ...event }
    // `docId` is repository metadata, never part of the persisted document.
    // Keeping it would let a later hydration overwrite the actual FYLO id and
    // could make a series cleanup target the same document twice.
    delete stored.docId
    stored.attendeesJson = JSON.stringify(Array.isArray(event.attendees) ? event.attendees : [])
    delete (/** @type {any} */ (stored)).attendees
    return await this.fylo.putData(Collections.CALENDAR_EVENTS, stored)
  }

  /**
   * List every event owned by a user, optionally clipped to an inclusive ISO
   * time window. Events that overlap the window in either direction are
   * included — we keep the overlap check in JS rather than expressing it
   * through Fylo query primitives.
   *
   * @param {string} userEmail
   * @param {{ start?: string, end?: string }} [range]
   * @returns {Promise<Array<CalendarEvent & { docId: string }>>}
   */
  async list(userEmail, range = {}) {
    const docs = await collect(
      this.fylo.findDocs(Collections.CALENDAR_EVENTS, { $ops: [{ userEmail: { $eq: userEmail } }] }).collect(),
    )
    /** @type {Array<CalendarEvent & { docId: string }>} */
    const out = Object.entries(docs).map(([docId, d]) => ({ docId, ...hydrate(/** @type {Record<string, unknown>} */ (d)) }))
    if (!range.start && !range.end) return out
    const windowStart = range.start ? Date.parse(range.start) : -Infinity
    const windowEnd = range.end ? Date.parse(range.end) : Infinity
    return out.filter(e => {
      // A recurring master may begin years before the requested window and
      // exceptions may move away from their original occurrence. The bounded
      // recurrence service needs both records to expand and replace instances.
      if (e.recurrence || e.seriesId) return true
      return CalendarDate.overlaps(e, windowStart, windowEnd)
    })
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, (CalendarEvent & { docId: string }) | null]>}
   */
  async findById(id) {
    const docs = await collect(
      this.fylo.findDocs(Collections.CALENDAR_EVENTS, { $ops: [{ id: { $eq: id } }] }).collect(),
    )
    const entry = Object.entries(docs)[0]
    if (!entry) return [null, null]
    return [entry[0], { docId: entry[0], ...hydrate(/** @type {Record<string, unknown>} */ (entry[1])) }]
  }

  /**
   * Find an event by ICS UID. Fylo's multi-condition `$ops` is OR'd, so we
   * narrow by `userEmail` and filter by `icsUid` in JS to avoid matching
   * manual events that have no `icsUid`.
   *
   * @param {string} userEmail
   * @param {string} icsUid
   * @returns {Promise<[string | null, (CalendarEvent & { docId: string }) | null]>}
   */
  async findByIcsUid(userEmail, icsUid) {
    const docs = await collect(
      this.fylo.findDocs(Collections.CALENDAR_EVENTS, { $ops: [{ userEmail: { $eq: userEmail } }] }).collect(),
    )
    for (const [docId, raw] of Object.entries(docs)) {
      if (/** @type {any} */ (raw)?.icsUid === icsUid && !/** @type {any} */ (raw)?.recurrenceId) {
        return [docId, { docId, ...hydrate(/** @type {Record<string, unknown>} */ (raw)) }]
      }
    }
    return [null, null]
  }

  /**
   * @param {string} userEmail
   * @param {string} idempotencyKey
   * @returns {Promise<[string | null, (CalendarEvent & {docId: string}) | null]>}
   */
  async findByIdempotencyKey(userEmail, idempotencyKey) {
    const events = await this.list(userEmail)
    const event = events.find(candidate => candidate.idempotencyKey === idempotencyKey)
    return event ? [event.docId, event] : [null, null]
  }

  /**
   * @param {string} seriesId
   * @param {string} recurrenceId
   * @returns {Promise<[string | null, (CalendarEvent & { docId: string }) | null]>}
   */
  async findException(seriesId, recurrenceId) {
    const docs = await collect(
      this.fylo.findDocs(Collections.CALENDAR_EVENTS, { $ops: [{ seriesId: { $eq: seriesId } }] }).collect(),
    )
    const normalizedDate = CalendarDate.isDate(recurrenceId) ? recurrenceId : ''
    const normalizedInstant = CalendarDate.isDateTime(recurrenceId) ? new Date(recurrenceId).toISOString() : ''
    if (!normalizedDate && !normalizedInstant) return [null, null]
    for (const [docId, raw] of Object.entries(docs)) {
      const value = /** @type {any} */ (raw)?.recurrenceId
      const matchesDate = normalizedDate && CalendarDate.coerce(value) === normalizedDate
      const matchesInstant = normalizedInstant && CalendarDate.isDateTime(value) && new Date(value).toISOString() === normalizedInstant
      if (matchesDate || matchesInstant) {
        return [docId, { docId, ...hydrate(/** @type {Record<string, unknown>} */ (raw)) }]
      }
    }
    return [null, null]
  }

  /**
   * Delete a master and all of its persisted exceptions for one exact owner.
   * @param {string} seriesId
   * @param {string} userEmail
   * @returns {Promise<number>}
   */
  async deleteSeries(seriesId, userEmail) {
    const events = await this.list(userEmail)
    const targets = events.filter(event => event.id === seriesId || event.seriesId === seriesId)
    const documentIds = [...new Set(targets.map(event => event.docId).filter(Boolean))]
    for (const documentId of documentIds) await this.delete(documentId)
    return documentIds.length
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    await this.fylo.delDoc(Collections.CALENDAR_EVENTS, docId)
  }
}

/**
 * @param {Record<string, unknown>} raw
 * @returns {CalendarEvent & { docId?: string }}
 */
function hydrate(raw) {
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
