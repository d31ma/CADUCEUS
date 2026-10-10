import { Respond } from '../../../../platform/http/respond.js'
import { getApp } from '../../../../app.js'

export class Handler {
  /** Public bearer-capability view; invalid, expired, and revoked are identical. */
  static async GET(/** @type {any} */ { paths }) {
    const app = await getApp()
    return await app.threadSharing.resolve(paths?.token)
      ?? Respond.r404('Shared conversation not found')
  }
}
