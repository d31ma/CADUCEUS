import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'

@RequestRoute("/thread-shares/public/{token}/attachments/{attachment_id}")
@Public
export class ThreadSharesPublicTokenAttachmentsAttachmentIdController {
  /** @type {ThreadSharingService} */
  @Needs
  threadSharingService

  @GetRoute
  async get(@FromPath token, @FromPath attachment_id, request) {
    return RouteRequest.handle(request, { paths: { token, attachment_id } }, async ({ paths }) => {
      const app = this
      return await app.threadSharingService.readAttachment(paths?.token, paths?.attachment_id)
        ?? Respond.r404('Shared attachment not found')
    })
  }
}
