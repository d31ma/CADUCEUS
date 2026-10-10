import { Guards } from '../../middleware/guards.js'
import { Respond } from '../../platform/http/respond.js'
import { getApp } from '../../app.js'

export class Handler {
  /** Return exact-owner physical categories and their assigned system roles. */
  @Guards.auth()
  static async GET(/** @type {any} */ { auth }) {
    const app = await getApp()
    const accounts = await app.categoryMappings.listAccounts(auth.email, auth.domains)
    if (!accounts) return Respond.r404('User not found')
    return { roles: ['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'], accounts }
  }

  /** Create a nested folder or label in the exact owned mailbox catalog. */
  @Guards.auth()
  static async POST(/** @type {any} */ { auth, body }) {
    const app = await getApp()
    const result = await app.categoryMappings.createCategory(auth.email, body)
    if (!result.ok) {
      if (result.notFound) return Respond.r404(result.error)
      if (result.forbidden) return Respond.r403(result.error)
      return Respond.r400(result.error)
    }
    return { category: result.category }
  }
}
