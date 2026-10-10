// @ts-check

import { Ics } from '../shared/calendar/Ics.js'
import { CalendarAttachments } from '../shared/calendar/CalendarAttachments.js'
import { CalendarImportRules } from '../shared/calendar/CalendarImportRules.js'

/** @import { CalendarEventsHelper } from '../helpers/CalendarEventsHelper.js' */
/** @import { CalendarEvent } from '../helpers/CalendarEventsHelper.js' */
/** @import { Attendee } from '../helpers/CalendarEventsHelper.js' */
/** @import { ParsedAttachment } from '../helpers/AttachmentsHelper.js' */
/** @import { ImportContext } from '../models/ImportContext.js' */
/** @import { ImportResult } from '../models/ImportResult.js' */

/**
 * Scan an inbound email for iCalendar payloads and upsert matching events on
 * the recipient's calendar.
 *
 * Per VEVENT, behaviour is keyed on the surrounding VCALENDAR METHOD:
 *   - REQUEST (or absent) — upsert by `icsUid`, mark `status: confirmed`
 *   - CANCEL — delete the matching event
 *   - REPLY — update the replier's attendee `status`
 *
 * Failures (parse errors, Fylo writes) are swallowed and logged; calendar
 * import is a nice-to-have side-effect on top of mail delivery and must never
 * block the parent flow.
 */
@Service
export default class CalendarImportService {
  /** @type {CalendarEventsHelper} */
  @Needs
  calendarEventsHelper

  /**
   * @param {ImportContext} ctx
   * @returns {Promise<ImportResult>}
   */
  async importFromInboundEmail(ctx) {
    const candidates = CalendarImportRules.collectIcsCandidates(ctx.textBody, ctx.attachments)
    const result = { imported: 0, cancelled: 0, replyApplied: 0 }

    for (const ics of candidates) {
      let cal
      try { cal = Ics.parseIcs(ics) } catch { continue }
      if (!cal || cal.events.length === 0) continue
      const method = cal.method || 'REQUEST'

      // Masters first so exceptions can attach to a stable series id even when
      // a provider serializes RECURRENCE-ID components before the master.
      for (const ev of [...cal.events].sort((left, right) => Number(!!left.recurrenceId) - Number(!!right.recurrenceId))) {
        try {
          if (method === 'CANCEL') {
            if (await this._cancel(ctx, ev)) result.cancelled++
          } else if (method === 'REPLY') {
            if (await this._applyReply(ctx, ev)) result.replyApplied++
          } else {
            if (await this._upsertRequest(ctx, ev)) result.imported++
          }
        } catch (err) {
          console.warn('[calendar-import] failed to import VEVENT', { uid: ev.uid, err: String(err) })
        }
      }
    }
    return result
  }

  /** @param {ImportContext} ctx @param {import('../shared/calendar/Ics.js').IcsEvent} ev */
  async _cancel(ctx, ev) {
    const [, master] = await this.calendarEventsHelper.findByIcsUid(ctx.recipient, ev.uid)
    if (!master) return false
    const now = new Date().toISOString()
    if (ev.recurrenceId) {
      const [existingDocId, existing] = await this.calendarEventsHelper.findException(master.id, ev.recurrenceId)
      if (existingDocId) await this.calendarEventsHelper.delete(existingDocId)
      await this.calendarEventsHelper.put(/** @type {any} */ ({
        ...(existing || master),
        id: existing?.id || `evt-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
        start: ev.start || existing?.start || ev.recurrenceId,
        end: ev.end || existing?.end || ev.recurrenceId,
        sourceEmailId: ctx.sourceEmailId,
        sourceEmailIds: CalendarImportRules.sourceEmailHistory(existing || master, ctx.sourceEmailId),
        seriesId: master.id,
        recurrenceId: ev.recurrenceId,
        recurrence: '',
        recurrenceDates: [],
        exceptionDates: [],
        status: 'cancelled',
        createdAt: existing?.createdAt || now,
        updatedAt: now,
      }))
      return true
    }
    const [docId] = await this.calendarEventsHelper.findByIcsUid(ctx.recipient, ev.uid)
    if (!docId) return false
    await this.calendarEventsHelper.delete(docId)
    await this.calendarEventsHelper.put({
      ...master,
      sourceEmailId: ctx.sourceEmailId,
      sourceEmailIds: CalendarImportRules.sourceEmailHistory(master, ctx.sourceEmailId),
      status: 'cancelled',
      updatedAt: now,
    })
    return true
  }

  /** @param {ImportContext} ctx @param {import('../shared/calendar/Ics.js').IcsEvent} ev */
  async _applyReply(ctx, ev) {
    const [docId, existing] = await this.calendarEventsHelper.findByIcsUid(ctx.recipient, ev.uid)
    if (!docId || !existing) return false
    const replier = ev.attendees[0]
    if (!replier) return false

    /** @type {Attendee[]} */
    const attendees = [...(existing.attendees || [])]
    const idx = attendees.findIndex(a => a.email.toLowerCase() === replier.email.toLowerCase())
    if (idx >= 0) attendees[idx] = { ...attendees[idx], status: replier.status }
    else attendees.push({ email: replier.email, name: replier.name, status: replier.status })

    // Delete the prior document first — putData always inserts under a fresh
    // docId, so re-putting without this leaves a stale copy behind and reads
    // can return the pre-RSVP attendee list.
    await this.calendarEventsHelper.delete(docId)
    await this.calendarEventsHelper.put({
      ...existing,
      attendees,
      sourceEmailId: ctx.sourceEmailId,
      sourceEmailIds: CalendarImportRules.sourceEmailHistory(existing, ctx.sourceEmailId),
      updatedAt: new Date().toISOString(),
    })
    return true
  }

  /** @param {ImportContext} ctx @param {import('../shared/calendar/Ics.js').IcsEvent} ev */
  async _upsertRequest(ctx, ev) {
    if (ev.recurrenceId) return await this._upsertException(ctx, ev)
    const [existingDocId, existing] = await this.calendarEventsHelper.findByIcsUid(ctx.recipient, ev.uid)
    const now = new Date().toISOString()
    /** @type {CalendarEvent} */
    const next = {
      id: existing?.id || `evt-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
      userEmail: ctx.recipient,
      title: ev.summary,
      start: ev.start,
      end: ev.end,
      allDay: !!ev.allDay,
      location: ev.location || '',
      description: ev.description || '',
      attendees: ev.attendees || [],
      organizer: ev.organizer || '',
      source: 'invite',
      sourceEmailId: ctx.sourceEmailId,
      sourceEmailIds: CalendarImportRules.sourceEmailHistory(existing, ctx.sourceEmailId),
      icsUid: ev.uid,
      recurrence: ev.rrule || '',
      recurrenceDates: ev.recurrenceDates || [],
      exceptionDates: ev.exceptionDates || [],
      timeZone: ev.timeZone || 'UTC',
      status: ev.status || 'confirmed',
      createdAt: existing?.createdAt || now,
      updatedAt: now,
    }
    // Delete the prior document first so we don't fragment under a new docId.
    if (existingDocId) await this.calendarEventsHelper.delete(existingDocId)
    await this.calendarEventsHelper.put(next)
    return true
  }

  /** @param {ImportContext} ctx @param {import('../shared/calendar/Ics.js').IcsEvent} ev */
  async _upsertException(ctx, ev) {
    const [, master] = await this.calendarEventsHelper.findByIcsUid(ctx.recipient, ev.uid)
    if (!master || !ev.recurrenceId) return false
    const [existingDocId, existing] = await this.calendarEventsHelper.findException(master.id, ev.recurrenceId)
    const now = new Date().toISOString()
    /** @type {CalendarEvent} */
    const next = {
      id: existing?.id || `evt-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`,
      userEmail: ctx.recipient,
      title: ev.summary || master.title,
      start: ev.start || master.start,
      end: ev.end || master.end,
      allDay: !!ev.allDay,
      location: ev.location || '',
      description: ev.description || '',
      attendees: ev.attendees || master.attendees || [],
      organizer: ev.organizer || master.organizer || '',
      source: 'invite',
      sourceEmailId: ctx.sourceEmailId,
      sourceEmailIds: CalendarImportRules.sourceEmailHistory(existing || master, ctx.sourceEmailId),
      icsUid: ev.uid,
      seriesId: master.id,
      recurrenceId: ev.recurrenceId,
      recurrence: '',
      recurrenceDates: [],
      exceptionDates: [],
      timeZone: ev.timeZone || master.timeZone || 'UTC',
      status: ev.status || 'confirmed',
      createdAt: existing?.createdAt || now,
      updatedAt: now,
    }
    if (existingDocId) await this.calendarEventsHelper.delete(existingDocId)
    await this.calendarEventsHelper.put(next)
    return true
  }
}

