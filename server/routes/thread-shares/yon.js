import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { Actions } from '../../platform/auth/bridge.js'
import { getApp } from '../../app.js'
import { ThreadShareInputError } from '../../features/mailbox/services/thread-sharing.js'

export class Handler {
  /** List exact-owner links, optionally narrowed to one authorized thread. */
  @Guards.auth({ action: Actions.MAILBOX_READ })
  static async GET(/** @type {any} */ { query, auth }) {
    const emailId = query?.emailId == null ? '' : String(query.emailId).trim()
    if (emailId.length > 500) return Respond.r400('emailId is too long')
    const app = await getApp()
    const result = await app.threadSharing.list(auth.email, emailId, auth.domains)
    if (result == null) return Respond.r404('Conversation not found')
    return result
  }

  /** Mint one expiring link and revoke any prior active link for the thread. */
  @Guards.auth({ action: Actions.MAILBOX_READ })
  static async POST(/** @type {any} */ { body, auth }) {
    try {
      const app = await getApp()
      const result = await app.threadSharing.create({
        ownerEmail: auth.email,
        domains: auth.domains,
        emailId: body?.emailId,
        expiresInDays: body?.expiresInDays,
      })
      return result ?? Respond.r404('Conversation not found')
    } catch (error) {
      if (error instanceof ThreadShareInputError) return Respond.r400(error.message)
      throw error
    }
  }
}
