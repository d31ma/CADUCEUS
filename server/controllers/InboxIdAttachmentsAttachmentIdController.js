import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { canAccessEmail } from '../features/mailbox/repositories/EmailsRepository.js'
import { parseExternalMessageId, readExternalAttachment } from '../shared/mailbox/external-message.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/inbox/{id}/attachments/{attachment_id}")
@SignedIn
export class InboxIdAttachmentsAttachmentIdController {
  /**
 * GET /inbox/_id/attachments/_attachment_id
 * @param {object} params
 * @param {{ id: string, attachment_id: string }} params.paths
 * @param {{ bearer?: { token: string } }} params.context
 * @returns {Promise<Record<string, unknown>>}
 */
  @GetRoute
  async get(@FromPath id, @FromPath attachment_id, request) {
    return RouteRequest.handle(request, { paths: { id, attachment_id }, guard: {} }, async ({ paths, auth, context }) => {
    const emailId = paths?.id
    const attachmentId = paths?.attachment_id
    if (!emailId || !attachmentId) return Respond.r404('Attachment not found')

    const external = parseExternalMessageId(emailId)
    if (external) {
      try {
        const source = await ExternalAccounts.messageSource(
          bearerToken(context), external.accountId, external.stableId,
        )
        const attachment = await readExternalAttachment(source?.rawRFC822Base64, attachmentId)
        return attachment || Respond.r404('Attachment not found')
      } catch (error) {
        return accountFailure(error)
      }
    }

    const app = await getApp()
    const [, email] = await app.emails.findById(emailId)
    if (!email) return Respond.r404('Email not found')
    if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
    if (!canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

    const [, attachment] = await app.attachments.findById(attachmentId)
    if (!attachment || attachment.emailId !== emailId) return Respond.r404('Attachment not found')

    const bytes = await app.attachments.readContent(attachment)
    return {
      id: attachment.id,
      filename: attachment.filename,
      contentType: attachment.contentType,
      size: attachment.size,
      contentBase64: Buffer.from(bytes).toString('base64'),
    }
    })
  }
}
