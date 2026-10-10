import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { canAccessEmail } from '../features/mailbox/repositories/EmailsRepository.js'
import { GitHubNotification } from '../shared/mailbox/GitHubNotification.js'
import { Ics } from '../shared/calendar/Ics.js'
import { CalendarAttachments } from '../shared/calendar/CalendarAttachments.js'
import { ExternalMessage } from '../shared/mailbox/ExternalMessage.js'
import { UnifiedMail } from '../shared/mailbox/UnifiedMail.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'
import { ExternalFolderError } from '../shared/mailbox/ExternalFolderError.js'

@RequestRoute("/inbox/{id}")
@SignedIn
export class InboxIdController {
  static SYSTEM_FOLDERS = new Set(['inbox', 'archive', 'trash'])

  /** @param {unknown} sender */
  static extractSenderEmail(sender) {
    const value = String(sender || '').trim().toLowerCase()
    const bracketed = value.match(/<([^<>\s]+@[^<>\s]+)>/)
    if (bracketed) return bracketed[1]
    const bare = value.match(/[a-z0-9.!#$%&\x27*+/=?^_\x60{|}~-]+@[a-z0-9.-]+/)
    return bare?.[0] || ''
  }

  /**
   * DELETE /inbox/_id
   * @param {object} params
   * @param {{ id: string }} params.paths
   * @param {{ bearer?: { token: string } }} params.context
   * @returns {Promise<Record<string, unknown>>}
   */
  @DeleteRoute
  async delete(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      const id = paths?.id
      if (!id) return Respond.r404('Email not found')
      if (InboxIdController.isExternalRoute(id)) {
        try {
          const loaded = await UnifiedMail.loadExternalMessage(ExternalAccounts.bearerToken(context), id)
          if (!loaded) return Respond.r404('Email not found')
          const mutation = await UnifiedMail.queueExternalMessageDelete(ExternalAccounts.bearerToken(context), loaded)
          return { deleted: id, pendingRemoteMutation: true, mutationId: mutation?.id || '' }
        } catch (error) {
          return ExternalAccounts.accountFailure(error)
        }
      }

      const app = await getApp()
      const [docId, email] = await app.emails.findById(id)
      if (!docId || !email) return Respond.r404('Email not found')
      if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
      if (!canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

      await app.emails.delete(docId, id)
      return { deleted: id }
    })
  }

  /**
   * PUT /inbox/_id
   *
   * Update one or more mutable fields of the email (`read`, `starred`, `folder`).
   *
   * @param {object} params
   * @param {{ read?: boolean, starred?: boolean, folder?: string }} params.body
   * @param {{ id: string }} params.paths
   * @param {{ bearer?: { token: string } }} params.context
   * @returns {Promise<Record<string, unknown>>}
   */
  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth, context }) => {
      const id = paths?.id
      if (!id) return Respond.r404('Email not found')

      const patch = InboxIdController.parsePatch(body)
      if (patch instanceof Error) return Respond.r400(patch.message)
      if (InboxIdController.isExternalRoute(id)) {
        try {
          const token = ExternalAccounts.bearerToken(context)
          const loaded = await UnifiedMail.loadExternalMessage(token, id)
          if (!loaded) return Respond.r404('Email not found')
          const mutations = await UnifiedMail.queueExternalMessagePatch(token, loaded, patch)
          const projected = UnifiedMail.projectExternalPatch(loaded, patch)
          const detail = projected
            ? await ExternalMessage.presentExternalDetail(projected.indexed, projected.rawRFC822Base64)
            : null
          if (!detail) return Respond.r404('Email not found')
          return {
            ...detail,
            calendarInvite: null,
            pendingRemoteMutation: true,
            mutationIds: mutations.map(mutation => mutation.id),
          }
        } catch (error) {
          if (error instanceof ExternalFolderError) return Respond.r400(error.message)
          return ExternalAccounts.accountFailure(error)
        }
      }

      const app = await getApp()
      const [docId, email] = await app.emails.findById(id)
      if (!docId || !email) return Respond.r404('Email not found')
      if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
      if (!canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

      const appliedPatch = { ...patch }
      if (typeof appliedPatch.folder === 'string') {
        appliedPatch.folder = await app.categoryMappings.pathFor(auth.email, email.domain, appliedPatch.folder) || appliedPatch.folder
      }
      await app.emails.update(docId, appliedPatch)
      const updated = { ...email, ...appliedPatch }
      const githubUrl = GitHubNotification.findTarget(updated)
      return await app.categoryMappings.presentEmail(auth.email, /** @type {any} */ ({
        ...updated,
        ...(githubUrl ? { githubUrl } : {}),
        hasRawSource: Boolean(await app.rawMessages.findForEmail(id)),
        attachments: await app.attachments.listSummaries(id),
      }))
    })
  }

  /**
   * GET /inbox/_id
   *
   * Return an email by ID with its attachment summaries and, when the email
   * originated from a meeting invite, the linked calendar event so the UI can
   * render an Accept/Decline/Tentative widget.
   *
   * @param {object} params
   * @param {{ id: string }} params.paths
   * @param {{ bearer?: { token: string } }} params.context
   * @returns {Promise<Record<string, unknown>>}
   */
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      const id = paths?.id
      if (!id) return Respond.r404('Email not found')
      if (InboxIdController.isExternalRoute(id)) {
        try {
          const loaded = await UnifiedMail.loadExternalMessage(ExternalAccounts.bearerToken(context), id)
          if (!loaded) return Respond.r404('Email not found')
          const detail = await ExternalMessage.presentExternalDetail(loaded.indexed, loaded.rawRFC822Base64)
          return detail ? { ...detail, calendarInvite: null } : Respond.r404('Email not found')
        } catch (error) {
          return ExternalAccounts.accountFailure(error)
        }
      }

      const app = await getApp()
      const [, email] = await app.emails.findById(id)
      if (!email) return Respond.r404('Email not found')
      if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
      if (!canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

      const presented = await app.domainMigration.presentEmail(email)
      const rawSource = await app.rawMessages.findForEmail(id)
      const githubUrl = GitHubNotification.findTarget(presented)
      const attachmentRecords = await app.attachments.list(id)
      const calendarAttachment = CalendarAttachments.bestCalendarAttachment(attachmentRecords)
      const calendarInvite = await InboxIdController.findCalendarInvite(app, auth.email, id, {
        email,
        attachment: calendarAttachment,
      })
      const visibleAttachments = CalendarAttachments.withoutCalendarCardAttachments(attachmentRecords, calendarInvite ? calendarAttachment : null)
        .map(({ id, filename, contentType, size, disposition, contentId }) => ({ id, filename, contentType, size, disposition, contentId }))
      return await app.categoryMappings.presentEmail(auth.email, /** @type {any} */ ({
        ...presented,
        ...(githubUrl ? { githubUrl } : {}),
        hasRawSource: Boolean(rawSource),
        attachments: visibleAttachments,
        calendarInvite,
      }))
    })
  }

  /** @param {unknown} input @returns {Record<string, unknown> | Error} */
  static parsePatch(input) {
    if (!input || typeof input !== 'object') return new Error('Update payload required')
    const body = /** @type {Record<string, unknown>} */ (input)
    /** @type {Record<string, unknown>} */
    const patch = {}

    if ('folder' in body) {
      if (typeof body.folder !== 'string' || !body.folder.trim()) {
        return new Error('folder must be a non-empty string')
      }
      const folder = body.folder.trim().toLowerCase()
      if (!InboxIdController.SYSTEM_FOLDERS.has(folder) && folder.length > 60) return new Error('folder is too long')
      patch.folder = folder
    }
    if ('read' in body) {
      if (typeof body.read !== 'boolean') return new Error('read must be a boolean')
      patch.read = body.read
    }
    if ('starred' in body) {
      if (typeof body.starred !== 'boolean') return new Error('starred must be a boolean')
      patch.starred = body.starred
    }

    if (Object.keys(patch).length === 0) return new Error('No supported fields to update')
    return patch
  }

  static isExternalRoute(value) {
    return typeof value === 'string' && value.startsWith('extmsg.') && Boolean(ExternalMessage.parseExternalMessageId(value))
  }

  /**
   * Look up the calendar event the inbound importer attached to this email,
   * if any. Returns `null` when this email isn't a meeting invite.
   *
   * @param {import('../app.js').App} app
   * @param {string} userEmail
   * @param {string} emailId
   */
  static async findCalendarInvite(app, userEmail, emailId, context = {}) {
    // We list the user's events and filter by `sourceEmailId` in JS rather than
    // pushing into Fylo's `$ops` — multi-condition `$ops` is OR'd, so the
    // narrow-then-filter pattern avoids false positives.
    const events = await app.calendarEvents.list(userEmail)
    const event = events.find(e => /** @type {any} */ (e).sourceEmailId === emailId ||
      (Array.isArray(/** @type {any} */ (e).sourceEmailIds) && /** @type {any} */ (e).sourceEmailIds.includes(emailId)))
    if (!event) return null

    const me = (event.attendees || []).find(a => (a?.email || '').toLowerCase() === userEmail.toLowerCase())
    const messageCalendar = await InboxIdController.readMessageCalendar(app, context.attachment)
    const messageEvent = messageCalendar?.events.find(candidate => candidate.uid === /** @type {any} */ (event).icsUid) || messageCalendar?.events[0]
    const messageMethod = messageCalendar?.method || (event.status === 'cancelled' ? 'CANCEL' : 'REQUEST')
    const senderEmail = InboxIdController.extractSenderEmail(context.email?.sender)
    const senderAttendee = messageMethod === 'REPLY'
      ? messageEvent?.attendees.find(attendee => attendee.email.toLowerCase() === senderEmail) || messageEvent?.attendees[0]
      : null
    const attachment = context.attachment
      ? {
          id: context.attachment.id,
          filename: context.attachment.filename,
          contentType: context.attachment.contentType,
          size: context.attachment.size,
        }
      : null
    return {
      id: event.id,
      title: event.title,
      start: event.start,
      end: event.end,
      allDay: !!event.allDay,
      location: event.location || '',
      description: event.description || '',
      organizer: event.organizer || '',
      attendees: event.attendees || [],
      status: event.status || 'confirmed',
      myStatus: me?.status || 'needsAction',
      icsUid: /** @type {any} */ (event).icsUid || '',
      recurrence: /** @type {any} */ (event).recurrence || '',
      recurrenceId: /** @type {any} */ (event).recurrenceId || '',
      isRecurring: !!(/** @type {any} */ (event).recurrence || /** @type {any} */ (event).seriesId),
      messageMethod,
      canRespond: messageMethod === 'REQUEST' && event.status !== 'cancelled',
      attachment,
      senderResponse: senderAttendee ? {
        email: senderAttendee.email,
        name: senderAttendee.name || senderAttendee.email,
        status: senderAttendee.status || 'needsAction',
      } : null,
    }
  }

  /**
   * Parse only the selected, access-controlled calendar attachment. A malformed
   * attachment must not make the containing email unreadable.
   * @param {import('../app.js').App} app
   * @param {any} attachment
   */
  static async readMessageCalendar(app, attachment) {
    if (!attachment) return null
    try {
      const bytes = await app.attachments.readContent(attachment)
      return Ics.parseIcs(new TextDecoder('utf-8').decode(bytes))
    } catch {
      return null
    }
  }
}
