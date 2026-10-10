import { Respond } from '../../../../../../platform/http/respond.js'
import { getApp } from '../../../../../../app.js'

export class Handler {
  static async GET(/** @type {any} */ { paths }) {
    const app = await getApp()
    return await app.threadSharing.readAttachment(paths?.token, paths?.attachment_id)
      ?? Respond.r404('Shared attachment not found')
  }
}
