// @ts-check

/**
 * Minimal iCalendar (RFC 5545) parser for inbound meeting invites. We extract
 * VEVENT records along with the surrounding VCALENDAR METHOD — REQUEST (new
 * invite), CANCEL, or REPLY (attendee response).
 *
 * Hand-rolled so the dependency-free policy keeps the install footprint small;
 * Outlook/Google/Apple invites use a narrow, predictable subset that's
 * straightforward to handle here. Recurrence identity, timezone, EXDATE, and
 * RDATE fields are preserved for the bounded recurrence service.
 *
 * @module ics
 */

import { CalendarRecurrence } from './CalendarRecurrence.js'
import { CalendarDate } from './CalendarDate.js'

export class Ics {
  static #PARTSTAT_MAP = /** @type {const} */ ({
    ACCEPTED: 'accepted',
    DECLINED: 'declined',
    TENTATIVE: 'tentative',
    'NEEDS-ACTION': 'needsAction',
    DELEGATED: 'delegated',
    COMPLETED: 'completed',
  })

  static #STATUS_MAP = /** @type {const} */ ({
    CONFIRMED: 'confirmed',
    CANCELLED: 'cancelled',
    TENTATIVE: 'tentative',
  })

  static #DAY_MS = 24 * 60 * 60 * 1000

  /**
   * Quick check: does this string look like an iCalendar payload? Used to decide
   * whether to bother running the full parser on an attachment body.
   *
   * @param {string} text
   * @returns {boolean}
   */
  static looksLikeIcs(text) {
    if (!text || typeof text !== 'string') return false
    // RFC 5545 §3.6 requires BEGIN:VCALENDAR at the start of every ICS file.
    return /^BEGIN:VCALENDAR/im.test(text)
  }

  /**
   * Parse a full VCALENDAR payload.
   *
   * @param {string} text
   * @returns {IcsCalendar | null} `null` when the payload isn't VCALENDAR.
   */
  static parseIcs(text) {
    if (!Ics.looksLikeIcs(text)) return null
    const lines = Ics.#unfoldLines(text)

    /** @type {IcsCalendar} */
    const cal = { method: null, prodid: '', events: [] }
    /** @type {Partial<IcsEvent> | null} */
    let current = null
    let inEvent = false

    for (const raw of lines) {
      const { name, params, value } = Ics.#parseLine(raw)

      if (name === 'BEGIN' && value.toUpperCase() === 'VEVENT') {
        inEvent = true
        current = { attendees: [], allDay: false }
        continue
      }
      if (name === 'END' && value.toUpperCase() === 'VEVENT') {
        if (current?.start && !current.end) current.end = Ics.#implicitEnd(current.start, !!current.allDay, current.duration)
        if (current && current.uid && current.summary && current.start && current.end) {
          cal.events.push(/** @type {IcsEvent} */ (current))
        }
        inEvent = false
        current = null
        continue
      }
      if (!inEvent) {
        if (name === 'METHOD') cal.method = /** @type {IcsCalendar['method']} */ (value.toUpperCase())
        if (name === 'PRODID') cal.prodid = value
        continue
      }
      if (current) Ics.#applyEventField(current, name, params, value)
    }

    return cal
  }

  /**
   * Build a small VCALENDAR REPLY payload acknowledging an attendee's RSVP.
   * Sent back to the organizer when the user clicks Accept/Decline/Tentative.
   *
   * @param {object} input
   * @param {string} input.uid
   * @param {string} input.organizer
   * @param {string} input.attendee
   * @param {string} [input.attendeeName]
   * @param {'accepted'|'declined'|'tentative'} input.status
   * @param {string} input.summary
   * @param {string} input.start
   * @param {string} input.end
   * @param {boolean} [input.allDay]
   * @returns {string}
   */
  static buildIcsReply({ uid, organizer, attendee, attendeeName, status, summary, start, end, allDay = false }) {
    const partstat = status === 'accepted' ? 'ACCEPTED' : status === 'declined' ? 'DECLINED' : 'TENTATIVE'
    const cn = attendeeName ? `;CN="${attendeeName.replace(/"/g, '\\"')}"` : ''
    return [
      'BEGIN:VCALENDAR',
      'VERSION:2.0',
      'PRODID:-//Caduceus//Mail Calendar//EN',
      'METHOD:REPLY',
      'BEGIN:VEVENT',
      `UID:${uid}`,
      `DTSTAMP:${Ics.#formatIcsDateTime(new Date().toISOString())}`,
      allDay ? `DTSTART;VALUE=DATE:${start.replaceAll('-', '')}` : `DTSTART:${Ics.#formatIcsDateTime(start)}`,
      allDay ? `DTEND;VALUE=DATE:${end.replaceAll('-', '')}` : `DTEND:${Ics.#formatIcsDateTime(end)}`,
      `SUMMARY:${summary}`,
      `ORGANIZER:mailto:${organizer}`,
      `ATTENDEE${cn};PARTSTAT=${partstat}:mailto:${attendee}`,
      'END:VEVENT',
      'END:VCALENDAR',
    ].join('\r\n')
  }

  /**
   * Encode one writable event as a standards-based CalDAV resource.
   * @param {Record<string, any>} event
   * @param {string} uid
   */
  static buildCalendarResource(event, uid) {
    if (!uid || /[\r\n\0]/.test(uid)) throw new Error('calendar UID is invalid')
    return [
      'BEGIN:VCALENDAR',
      'VERSION:2.0',
      'PRODID:-//Caduceus//Mail Calendar//EN',
      'CALSCALE:GREGORIAN',
      ...Ics.#eventLines(event, uid, event.recurrenceId || '', 0),
      'END:VCALENDAR',
      '',
    ].flatMap(Ics.#foldLine).join('\r\n')
  }

  /**
   * Replace a master/exception while preserving unrelated VEVENTs, VTIMEZONE,
   * alarms, and provider-specific properties in the same resource.
   * @param {string} raw
   * @param {{ uid:string, recurrenceId?:string, event:Record<string, any> }} input
   */
  static updateEventResource(raw, input) {
    const document = Ics.parseIcs(raw)
    if (!document) throw new Error('provider calendar resource is invalid')
    const lines = Ics.#unfoldLines(raw)
    const blocks = Ics.#eventBlocks(lines)
    const recurrenceId = input.recurrenceId || ''
    const block = blocks.find(candidate => candidate.event?.uid === input.uid && (candidate.event?.recurrenceId || '') === recurrenceId)
    if (block) {
      const sequence = Number.isInteger(block.event?.sequence) ? Number(block.event.sequence) + 1 : 1
      const replacement = Ics.#mergeEventBlock(lines.slice(block.start, block.end + 1), input.event, input.uid, recurrenceId, sequence)
      lines.splice(block.start, block.end - block.start + 1, ...replacement)
    } else {
      if (!recurrenceId || !blocks.some(candidate => candidate.event?.uid === input.uid && !candidate.event?.recurrenceId)) {
        throw new Error('provider calendar event was not found')
      }
      const end = lines.lastIndexOf('END:VCALENDAR')
      if (end < 0) throw new Error('provider calendar resource is invalid')
      lines.splice(end, 0, ...Ics.#eventLines(input.event, input.uid, recurrenceId, 1))
    }
    return [...lines, ''].flatMap(Ics.#foldLine).join('\r\n')
  }

  /**
   * Remove a whole UID or exclude one recurrence instance. Returns null when
   * the entire CalDAV resource should be deleted.
   * @param {string} raw
   * @param {{ uid:string, recurrenceId?:string }} input
   * @returns {string | null}
   */
  static deleteEventResource(raw, input) {
    const document = Ics.parseIcs(raw)
    if (!document) throw new Error('provider calendar resource is invalid')
    const recurrenceId = input.recurrenceId || ''
    if (recurrenceId) {
      const master = document.events.find(event => event.uid === input.uid && !event.recurrenceId)
      if (!master) throw new Error('provider calendar series was not found')
      return Ics.updateEventResource(raw, {
        uid: input.uid,
        event: { ...master, recurrence: master.rrule || '', exceptionDates: [...new Set([...(master.exceptionDates || []), recurrenceId])] },
      })
    }
    const lines = Ics.#unfoldLines(raw)
    const blocks = Ics.#eventBlocks(lines).filter(block => block.event?.uid === input.uid).sort((left, right) => right.start - left.start)
    if (!blocks.length) throw new Error('provider calendar event was not found')
    for (const block of blocks) lines.splice(block.start, block.end - block.start + 1)
    if (!Ics.#eventBlocks(lines).length) return null
    return [...lines, ''].flatMap(Ics.#foldLine).join('\r\n')
  }

  /**
   * Apply a single VEVENT property to the in-progress event record.
   *
   * @param {Partial<IcsEvent>} event
   * @param {string} name
   * @param {Record<string, string>} params
   * @param {string} value
   */
  static #applyEventField(event, name, params, value) {
    switch (name) {
      case 'UID': event.uid = value; return
      case 'SUMMARY': event.summary = Ics.#unescapeText(value); return
      case 'DESCRIPTION': event.description = Ics.#unescapeText(value); return
      case 'LOCATION': event.location = Ics.#unescapeText(value); return
      case 'DTSTART':
        event.timeZone = params.TZID && CalendarRecurrence.isTimeZone(params.TZID) ? params.TZID : 'UTC'
        event.allDay = params.VALUE === 'DATE'
        event.start = event.allDay ? CalendarDate.fromCompact(value) : Ics.#parseIcsDateTime(value, event.timeZone)
        return
      case 'DTEND':
        event.end = params.VALUE === 'DATE'
          ? CalendarDate.fromCompact(value)
          : Ics.#parseIcsDateTime(value, params.TZID || event.timeZone || 'UTC')
        if (params.VALUE === 'DATE') event.allDay = true
        return
      case 'DTSTAMP': event.dtstamp = Ics.#parseIcsDateTime(value, params.TZID || 'UTC'); return
      case 'ORGANIZER': event.organizer = Ics.#extractEmail(value); return
      case 'ATTENDEE': {
        const email = Ics.#extractEmail(value)
        if (!email) return
        const status = Ics.#PARTSTAT_MAP[/** @type {keyof typeof PARTSTAT_MAP} */ (params.PARTSTAT)] || 'needsAction'
        event.attendees = event.attendees || []
        event.attendees.push({ email, name: params.CN || '', status })
        return
      }
      case 'RRULE': event.rrule = value; return
      case 'RDATE':
        event.recurrenceDates = [...(event.recurrenceDates || []), ...Ics.#parseDateList(value, params.TZID || event.timeZone || 'UTC', params.VALUE === 'DATE')]
        return
      case 'EXDATE':
        event.exceptionDates = [...(event.exceptionDates || []), ...Ics.#parseDateList(value, params.TZID || event.timeZone || 'UTC', params.VALUE === 'DATE')]
        return
      case 'RECURRENCE-ID':
        event.recurrenceId = params.VALUE === 'DATE'
          ? CalendarDate.fromCompact(value)
          : Ics.#parseIcsDateTime(value, params.TZID || event.timeZone || 'UTC')
        if (params.RANGE === 'THISANDFUTURE') event.recurrenceRange = 'THISANDFUTURE'
        return
      case 'SEQUENCE': if (/^\d+$/.test(value)) event.sequence = Number(value); return
      case 'DURATION': event.duration = value; return
      case 'STATUS':
        event.status = Ics.#STATUS_MAP[/** @type {keyof typeof STATUS_MAP} */ (value.toUpperCase())] || undefined
        return
    }
  }

  /**
   * Unfold RFC 5545 line continuations: lines starting with space or tab are
   * appended to the previous logical line.
   *
   * @param {string} text
   * @returns {string[]}
   */
  static #unfoldLines(text) {
    const raw = text.replace(/\r\n?|\n/g, '\n').split('\n')
    /** @type {string[]} */
    const lines = []
    for (const line of raw) {
      if ((line.startsWith(' ') || line.startsWith('\t')) && lines.length > 0) {
        lines[lines.length - 1] += line.slice(1)
      } else {
        lines.push(line)
      }
    }
    return lines.filter(l => l.length > 0)
  }

  /**
   * Split a content-line into `(name, paramsMap, value)`. Grammar (simplified):
   * `NAME [;PARAM=VALUE]* :VALUE`. Values may be quoted with `"..."` to escape
   * semicolons and colons.
   *
   * @param {string} line
   * @returns {{ name: string, params: Record<string, string>, value: string }}
   */
  static #parseLine(line) {
    let i = 0
    while (i < line.length && line[i] !== ';' && line[i] !== ':') i++
    const name = line.slice(0, i).toUpperCase()

    /** @type {Record<string, string>} */
    const params = {}
    while (line[i] === ';') {
      i++
      const pStart = i
      while (i < line.length && line[i] !== '=') i++
      const pName = line.slice(pStart, i).toUpperCase()
      i++

      let pVal
      if (line[i] === '"') {
        i++
        const vStart = i
        while (i < line.length && line[i] !== '"') i++
        pVal = line.slice(vStart, i)
        i++
      } else {
        const vStart = i
        while (i < line.length && line[i] !== ';' && line[i] !== ':') i++
        pVal = line.slice(vStart, i)
      }
      params[pName] = pVal
    }
    if (line[i] === ':') i++
    return { name, params, value: line.slice(i) }
  }

  /**
   * Convert ICS DATE / DATE-TIME representations to ISO 8601 in UTC.
   * TZID parameters are honoured nominally — local clock-time is kept and
   * treated as UTC for storage.
   *
   * @param {string} raw
   * @returns {string}
   */
  static #parseIcsDateTime(raw, timeZone = 'UTC') {
    return CalendarRecurrence.parseDateTime(raw, timeZone)
  }

  /** @param {string} value @param {string} timeZone @param {boolean} asDate */
  static #parseDateList(value, timeZone, asDate) {
    return value.split(',')
      .map(item => asDate ? CalendarDate.fromCompact(item) : Ics.#parseIcsDateTime(item, timeZone))
      .filter(item => asDate ? CalendarDate.isDate(item) : CalendarDate.isDateTime(item))
  }

  /** @param {string} start @param {boolean} allDay @param {string | undefined} duration */
  static #implicitEnd(start, allDay, duration) {
    if (allDay && CalendarDate.isDate(start)) {
      const days = Math.max(1, Math.round((Ics.#durationMilliseconds(duration) || Ics.#DAY_MS) / Ics.#DAY_MS))
      return CalendarDate.addDays(start, days)
    }
    const base = Date.parse(start)
    const durationMs = Ics.#durationMilliseconds(duration) || (allDay ? 24 * 60 * 60 * 1000 : 0)
    return new Date(base + durationMs).toISOString()
  }

  /** @param {string | undefined} value */
  static #durationMilliseconds(value) {
    if (!value) return 0
    const match = value.match(/^P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$/i)
    if (!match) return 0
    const [, weeks = '0', days = '0', hours = '0', minutes = '0', seconds = '0'] = match
    return (((Number(weeks) * 7 + Number(days)) * 24 + Number(hours)) * 60 * 60 + Number(minutes) * 60 + Number(seconds)) * 1000
  }

  /** @param {string} iso @returns {string} */
  static #formatIcsDateTime(iso) {
    const d = new Date(iso)
    const pad = (/** @type {number} */ n) => String(n).padStart(2, '0')
    return `${d.getUTCFullYear()}${pad(d.getUTCMonth() + 1)}${pad(d.getUTCDate())}` +
      `T${pad(d.getUTCHours())}${pad(d.getUTCMinutes())}${pad(d.getUTCSeconds())}Z`
  }

  /** @param {string} value */
  static #formatDate(value) { return String(value || '').replaceAll('-', '').slice(0, 8) }

  /** @param {string} value */
  static #escapeText(value) {
    return String(value || '').replace(/\\/g, '\\\\').replace(/\r?\n/g, '\\n').replace(/;/g, '\\;').replace(/,/g, '\\,')
  }

  /** @param {string} value */
  static #escapeParam(value) { return String(value || '').replace(/["\r\n]/g, '') }

  /** @param {Record<string, any>} event @param {string} uid @param {string} recurrenceId @param {number} sequence */
  static #eventLines(event, uid, recurrenceId, sequence) {
    const allDay = event.allDay === true
    const lines = ['BEGIN:VEVENT', `UID:${uid}`, `DTSTAMP:${Ics.#formatIcsDateTime(new Date().toISOString())}`, `SEQUENCE:${Math.max(0, sequence)}`]
    if (recurrenceId) lines.push(allDay ? `RECURRENCE-ID;VALUE=DATE:${Ics.#formatDate(recurrenceId)}` : `RECURRENCE-ID:${Ics.#formatIcsDateTime(recurrenceId)}`)
    lines.push(allDay ? `DTSTART;VALUE=DATE:${Ics.#formatDate(event.start)}` : `DTSTART:${Ics.#formatIcsDateTime(event.start)}`)
    lines.push(allDay ? `DTEND;VALUE=DATE:${Ics.#formatDate(event.end)}` : `DTEND:${Ics.#formatIcsDateTime(event.end)}`)
    lines.push(`SUMMARY:${Ics.#escapeText(event.title || event.summary || 'Untitled event')}`)
    if (event.location) lines.push(`LOCATION:${Ics.#escapeText(event.location)}`)
    if (event.description) lines.push(`DESCRIPTION:${Ics.#escapeText(event.description)}`)
    if (event.organizer) lines.push(`ORGANIZER:mailto:${String(event.organizer).toLowerCase()}`)
    const partstat = { accepted: 'ACCEPTED', declined: 'DECLINED', tentative: 'TENTATIVE', needsAction: 'NEEDS-ACTION', delegated: 'DELEGATED', completed: 'COMPLETED' }
    for (const attendee of Array.isArray(event.attendees) ? event.attendees : []) {
      if (!attendee?.email) continue
      const parameters = [`PARTSTAT=${partstat[attendee.status] || 'NEEDS-ACTION'}`]
      if (attendee.name) parameters.unshift(`CN="${Ics.#escapeParam(attendee.name)}"`)
      lines.push(`ATTENDEE;${parameters.join(';')}:mailto:${String(attendee.email).toLowerCase()}`)
    }
    const recurrence = event.recurrence || event.rrule || ''
    if (recurrence) lines.push(`RRULE:${recurrence}`)
    /** @param {string[]} values */
    const formatList = values => values.map(value => allDay ? Ics.#formatDate(value) : Ics.#formatIcsDateTime(value)).join(',')
    if (Array.isArray(event.recurrenceDates) && event.recurrenceDates.length) lines.push(`${allDay ? 'RDATE;VALUE=DATE' : 'RDATE'}:${formatList(event.recurrenceDates)}`)
    if (Array.isArray(event.exceptionDates) && event.exceptionDates.length) lines.push(`${allDay ? 'EXDATE;VALUE=DATE' : 'EXDATE'}:${formatList(event.exceptionDates)}`)
    if (event.status) lines.push(`STATUS:${String(event.status).toUpperCase()}`)
    lines.push('END:VEVENT')
    return lines
  }

  /** @param {string[]} lines */
  static #eventBlocks(lines) {
    const blocks = []
    let start = -1
    let depth = 0
    for (let index = 0; index < lines.length; index++) {
      const line = lines[index].toUpperCase()
      if (line === 'BEGIN:VEVENT') {
        if (depth === 0) start = index
        depth++
      } else if (line === 'END:VEVENT' && depth > 0) {
        depth--
        if (depth === 0 && start >= 0) {
          const wrapped = ['BEGIN:VCALENDAR', ...lines.slice(start, index + 1), 'END:VCALENDAR'].join('\r\n')
          blocks.push({ start, end: index, event: Ics.parseIcs(wrapped)?.events?.[0] || null })
          start = -1
        }
      }
    }
    return blocks
  }

  /** @param {string[]} block @param {Record<string, any>} event @param {string} uid @param {string} recurrenceId @param {number} sequence */
  static #mergeEventBlock(block, event, uid, recurrenceId, sequence) {
    const managed = new Set(['UID', 'DTSTAMP', 'SEQUENCE', 'RECURRENCE-ID', 'DTSTART', 'DTEND', 'SUMMARY', 'LOCATION', 'DESCRIPTION', 'ORGANIZER', 'ATTENDEE', 'RRULE', 'RDATE', 'EXDATE', 'STATUS'])
    const preserved = []
    let nested = 0
    for (const line of block.slice(1, -1)) {
      const { name, value } = Ics.#parseLine(line)
      if (name === 'BEGIN') nested++
      if (nested > 0 || !managed.has(name)) preserved.push(line)
      if (name === 'END' && value.toUpperCase() !== 'VEVENT') nested = Math.max(0, nested - 1)
    }
    const encoded = Ics.#eventLines(event, uid, recurrenceId, sequence)
    return [encoded[0], ...encoded.slice(1, -1), ...preserved, encoded.at(-1)]
  }

  /** @param {string} line @returns {string[]} */
  static #foldLine(line) {
    if (new TextEncoder().encode(line).byteLength <= 75) return [line]
    const output = []
    let remaining = line
    let limit = 75
    while (new TextEncoder().encode(remaining).byteLength > limit) {
      let boundary = 0
      for (const character of remaining) {
        if (new TextEncoder().encode(remaining.slice(0, boundary) + character).byteLength > limit) break
        boundary += character.length
      }
      output.push((output.length ? ' ' : '') + remaining.slice(0, boundary))
      remaining = remaining.slice(boundary)
      limit = 74
    }
    output.push((output.length ? ' ' : '') + remaining)
    return output
  }

  /** @param {string} v @returns {string} */
  static #unescapeText(v) {
    return v
      .replace(/\\n/gi, '\n')
      .replace(/\\,/g, ',')
      .replace(/\\;/g, ';')
      .replace(/\\\\/g, '\\')
  }

  /** @param {string} v @returns {string} */
  static #extractEmail(v) {
    if (!v) return ''
    return v.trim().replace(/^mailto:/i, '').toLowerCase()
  }
}
