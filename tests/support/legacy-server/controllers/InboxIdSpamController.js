import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { EmailsRecords } from '../shared/mailbox/EmailsRecords.js'

@RequestRoute("/inbox/{id}/spam")
@SignedIn
export class InboxIdSpamController {
  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /**
   * PUT /inbox/_id/spam
   *
   * Apply user spam feedback to an existing email. Reporting moves the message
   * into Spam; marking not-spam restores it to Inbox and clears spam metadata.
   *
   * @param {object} params
   * @param {{ id: string }} params.paths
   * @param {{ spam: boolean }} params.body
   * @param {import('../shared/http/AuthGuard.js').Principal} params.auth
   * @returns {Promise<Record<string, unknown>>}
   */
  @PutRoute
  async put(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, body, auth }) => {
      const id = paths?.id
      if (!id) return Respond.r404('Email not found')
      if (typeof body?.spam !== 'boolean') return Respond.r400('spam must be a boolean')

      const app = this
      const [docId, email] = await app.emailsHelper.findById(id)
      if (!docId || !email) return Respond.r404('Email not found')
      if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
      if (!EmailsRecords.canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

      const folder = await app.categoryMappingService.pathFor(auth.email, email.domain, body.spam ? 'spam' : 'inbox')
      const patch = body.spam
        ? { folder: folder || 'spam', spamReasons: ['user-reported'] }
        : { folder: folder || 'inbox', spamScore: null, spamReasons: null }

      await app.emailsHelper.update(docId, patch)
      const updated = /** @type {any} */ ({
        ...email,
        ...patch,
        attachments: await app.attachmentsHelper.listSummaries(id),
      })
      if (!body.spam) {
        delete updated.spamScore
        delete updated.spamReasons
      }
      return await app.categoryMappingService.presentEmail(auth.email, updated)
    })
  }
}
