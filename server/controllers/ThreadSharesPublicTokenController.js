import { Respond } from '../../../../platform/http/respond.js'
import { getApp } from '../../../../app.js'

@RequestRoute("/thread-shares/public/{token}")
@Public
export class ThreadSharesPublicTokenController {
  /** Public bearer-capability view; invalid, expired, and revoked are identical. */
  @GetRoute
  async get(/** @type {any} */ { paths }) {
    const app = await getApp()
    return await app.threadSharing.resolve(paths?.token)
      ?? Respond.r404('Shared conversation not found')
  }
}
