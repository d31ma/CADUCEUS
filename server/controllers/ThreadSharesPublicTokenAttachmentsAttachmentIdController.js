import { Respond } from '../../../../../../platform/http/respond.js'
import { getApp } from '../../../../../../app.js'

@RequestRoute("/thread-shares/public/{token}/attachments/{attachment_id}")
@Public
export class ThreadSharesPublicTokenAttachmentsAttachmentIdController {
  @GetRoute
  async get(/** @type {any} */ { paths }) {
    const app = await getApp()
    return await app.threadSharing.readAttachment(paths?.token, paths?.attachment_id)
      ?? Respond.r404('Shared attachment not found')
  }
}
