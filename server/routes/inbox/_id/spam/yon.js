import { Guards } from '../../../../middleware/guards.js'
import { Respond } from '../../../../platform/http/respond.js'
import { getApp } from '../../../../app.js'
import { canAccessEmail } from '../../../../features/mailbox/repositories/emails.js'

export class Handler {
  /**
   * PUT /inbox/_id/spam
   *
   * Apply user spam feedback to an existing email. Reporting moves the message
   * into Spam; marking not-spam restores it to Inbox and clears spam metadata.
   *
   * @param {object} params
   * @param {{ id: string }} params.paths
   * @param {{ spam: boolean }} params.body
   * @param {import('../../../../middleware/require-auth.js').Principal} params.auth
   * @returns {Promise<Record<string, unknown>>}
   */
  @Guards.auth()
  static async PUT(/** @type {any} */ { paths, body, auth }) {
    const id = paths?.id
    if (!id) return Respond.r404('Email not found')
    if (typeof body?.spam !== 'boolean') return Respond.r400('spam must be a boolean')

    const app = await getApp()
    const [docId, email] = await app.emails.findById(id)
    if (!docId || !email) return Respond.r404('Email not found')
    if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
    if (!canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

    const folder = await app.categoryMappings.pathFor(auth.email, email.domain, body.spam ? 'spam' : 'inbox')
    const patch = body.spam
      ? { folder: folder || 'spam', spamReasons: ['user-reported'] }
      : { folder: folder || 'inbox', spamScore: null, spamReasons: null }

    await app.emails.update(docId, patch)
    const updated = /** @type {any} */ ({
      ...email,
      ...patch,
      attachments: await app.attachments.listSummaries(id),
    })
    if (!body.spam) {
      delete updated.spamScore
      delete updated.spamReasons
    }
    return await app.categoryMappings.presentEmail(auth.email, updated)
  }
}
