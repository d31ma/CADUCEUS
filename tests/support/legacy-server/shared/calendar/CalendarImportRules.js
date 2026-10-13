// @ts-check

import { Ics } from './Ics.js'
import { CalendarAttachments } from './CalendarAttachments.js'

export class CalendarImportRules {
  /** @param {Partial<CalendarEvent> | null | undefined} event @param {string} current */
  static sourceEmailHistory(event, current) {
    const values = [...(Array.isArray(event?.sourceEmailIds) ? event.sourceEmailIds : []), event?.sourceEmailId, current]
      .filter(/** @returns {value is string} */ value => typeof value === 'string' && value.length > 0)
    return [...new Set(values)].slice(-32)
  }

  /**
   * Collect every iCalendar payload from an email — both inline in the text
   * body and as attachments with a calendar MIME type, `.ics`, or legacy `.vcs`
   * filename. Duplicate provider representations are parsed only once.
   *
   * @param {string} textBody
   * @param {ParsedAttachment[]} attachments
   * @returns {string[]}
   */
  static collectIcsCandidates(textBody, attachments) {
    /** @type {string[]} */
    const out = []
    const seen = new Set()
    if (Ics.looksLikeIcs(textBody)) {
      out.push(textBody)
      seen.add(textBody)
    }
    for (const a of attachments || []) {
      if (!CalendarAttachments.isCalendarAttachment(a)) continue
      try {
        const text = new TextDecoder('utf-8').decode(a.content || new Uint8Array())
        if (Ics.looksLikeIcs(text) && !seen.has(text)) {
          out.push(text)
          seen.add(text)
        }
      } catch {
        // ignore decode failures
      }
    }
    return out
  }
}
