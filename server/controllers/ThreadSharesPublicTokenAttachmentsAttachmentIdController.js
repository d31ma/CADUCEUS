import { RouteRequest } from '../middleware/RouteRequest.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'

@RequestRoute("/thread-shares/public/{token}/attachments/{attachment_id}")
@Public
export class ThreadSharesPublicTokenAttachmentsAttachmentIdController {
  @GetRoute
  async get(@FromPath token, @FromPath attachment_id, request) {
    return RouteRequest.handle(request, { paths: { token, attachment_id } }, async ({ paths }) => {
      const app = await getApp()
      return await app.threadSharing.readAttachment(paths?.token, paths?.attachment_id)
        ?? Respond.r404('Shared attachment not found')
    })
  }
}
