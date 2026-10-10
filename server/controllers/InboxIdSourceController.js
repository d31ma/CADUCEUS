import { Buffer } from 'node:buffer'
import { createHash } from 'node:crypto'
import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Respond } from '../shared/http/Respond.js'
import { getApp } from '../app.js'
import { canAccessEmail } from '../features/mailbox/repositories/EmailsRepository.js'
import { ExternalMessage } from '../shared/mailbox/ExternalMessage.js'
import { ExternalAccounts } from '../shared/accounts/ExternalAccounts.js'

@RequestRoute("/inbox/{id}/source")
@SignedIn
export class InboxIdSourceController {
  /** @param {string} value */
  static safeFilename(value) {
    const filename = String(value).replace(/[/\\?%*:|\x22<>\u0000-\u001f\u007f]/g, '-').replace(/\s+/g, ' ').trim()
    return filename.slice(0, 120) || 'message'
  }

  /**
   * GET /inbox/:id/source — return the exact RFC 822 source as an authenticated
   * base64 payload. The browser can inspect it without executing its markup or
   * download it as a standards-compatible `.eml` file.
   *
   * @param {any} request
   */
  @GetRoute
  async get(@FromPath id, request) {
    return RouteRequest.handle(request, { paths: { id }, guard: {} }, async ({ paths, auth, context }) => {
      const emailId = String(paths?.id || '')
      if (!emailId) return Respond.r404('Message source not found')

      const external = ExternalMessage.parseExternalMessageId(emailId)
      if (external) {
        try {
          const source = await ExternalAccounts.messageSource(
            ExternalAccounts.bearerToken(context), external.accountId, external.stableId,
          )
          const encoded = source?.rawRFC822Base64
          if (typeof encoded !== 'string' || encoded.length > 35_000_000) {
            return Respond.r404('Message source not found')
          }
          const bytes = Buffer.from(encoded, 'base64')
          if (bytes.length < 1 || bytes.length > 25 * 1024 * 1024) {
            return Respond.r404('Message source not found')
          }
          return {
            filename: `${InboxIdSourceController.safeFilename(source?.message?.subject || 'message')}.eml`,
            contentType: 'message/rfc822',
            size: bytes.length,
            sha256: createHash('sha256').update(bytes).digest('hex'),
            contentBase64: bytes.toString('base64'),
          }
        } catch (error) {
          return ExternalAccounts.accountFailure(error)
        }
      }

      const app = await getApp()
      const [, email] = await app.emails.findById(emailId)
      if (!email) return Respond.r404('Email not found')
      if (!auth.domains.includes(email.domain)) return Respond.r403('Access denied')
      if (!canAccessEmail(email, auth.email)) return Respond.r404('Email not found')

      const source = await app.rawMessages.findForEmail(emailId)
      if (!source) return Respond.r404('Message source not found')
      const bytes = await app.rawMessages.readContent(source)
      return {
        filename: `${InboxIdSourceController.safeFilename(email.subject || 'message')}.eml`,
        contentType: 'message/rfc822',
        size: bytes.byteLength,
        sha256: source.sha256,
        contentBase64: Buffer.from(bytes).toString('base64'),
      }
    })
  }
}
