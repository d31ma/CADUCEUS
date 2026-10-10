import { Buffer } from 'node:buffer'
import { RouteRequest } from '../middleware/RouteRequest.js'
import { AuthGuard } from '../middleware/AuthGuard.js'
import { FyloAccessContext } from '../platform/persistence/FyloAccessContext.js'
import { Respond } from '../platform/http/Respond.js'
import { getApp } from '../app.js'
import { ExternalAccounts, accountFailure, bearerToken } from '../platform/accounts/ExternalAccounts.js'
import { resolveExternalFolder } from '../features/mailbox/services/unified-mail.js'

/**
 * Thin migration adapter that streams the exact RFC 822 sources in one folder.
 * The browser owns mboxrd or EML-archive assembly, progress, destination
 * safety, and cancellation, matching Mailspring's client-side assembly
 * boundary. Each binary frame is:
 *
 *   uint32be metadataLength | uint32be sourceLength | metadata JSON | source
 *
 * The response starts with {@link MAGIC}. Only one message is resident at a
 * time, and a canceled response stops before reading the next FYLO bucket.
 */
@RequestRoute("/inbox/export/rfc822")
@SignedIn
export class InboxExportRfc822Controller {
  static MAGIC = Buffer.from('CADUCEUS-RFC822-FRAMES/1\n', 'ascii')

  static MAX_METADATA_BYTES = 16 * 1024

  @GetRoute
  async get(request) {
    return RouteRequest.handle(request, { guard: {} }, async ({ query, context, auth }) => {
      const accountId = String(query?.account || '').trim()
      const folder = accountId ? InboxExportRfc822Controller._providerFolder(query?.folder) : InboxExportRfc822Controller._folder(query?.folder)
      if (folder instanceof Error) return Respond.r400(folder.message)

      let iterator
      if (accountId) {
        try {
          const token = bearerToken(context)
          const response = await ExternalAccounts.folders(token, accountId)
          const providerFolder = resolveExternalFolder(response?.folders, folder)
          if (!providerFolder) return Respond.r404('Connected mailbox folder not found')
          iterator = InboxExportRfc822Controller._providerRecords(token, accountId, providerFolder.stableId)
        } catch (error) {
          return accountFailure(error)
        }
      } else {
        const requestedDomain = String(query?.domain || auth.domain || '').trim().toLowerCase()
        if (!requestedDomain) return Respond.r400('domain is required')
        if (requestedDomain !== auth.domain) {
          const permission = await AuthGuard.requireDomainAccess(context, requestedDomain)
          if (!permission.ok) return permission.response
        }
        const app = await getApp()
        const mappedFolder = await app.categoryMappings.pathFor(auth.email, requestedDomain, folder) || folder
        const access = { email: auth.email, domains: [requestedDomain] }
        iterator = InboxExportRfc822Controller._hostedRecords(app, access, requestedDomain, mappedFolder, auth.email)
      }
      let sentMagic = false
      let cancelled = false

      const body = new ReadableStream({
        async pull(controller) {
          if (cancelled) return
          if (!sentMagic) {
            sentMagic = true
            controller.enqueue(InboxExportRfc822Controller.MAGIC)
            return
          }

          try {
            const next = await iterator.next()
            if (next.done) {
              controller.close()
              return
            }
            const { email, source } = next.value
            if (cancelled) return
            if (source.byteLength > 0xffff_ffff) throw new Error('RFC 822 source exceeds the export frame limit')

            const metadata = Buffer.from(JSON.stringify({
              id: email.id,
              sender: email.sender,
              subject: email.subject,
              receivedAt: email.receivedAt,
            }), 'utf8')
            if (metadata.byteLength > InboxExportRfc822Controller.MAX_METADATA_BYTES) throw new Error('Export metadata exceeds its limit')
            const header = Buffer.allocUnsafe(8)
            header.writeUInt32BE(metadata.byteLength, 0)
            header.writeUInt32BE(source.byteLength, 4)
            controller.enqueue(header)
            controller.enqueue(metadata)
            controller.enqueue(source)
          } catch (error) {
            controller.error(error)
          }
        },
        async cancel() {
          cancelled = true
          await iterator.return?.(undefined)
        },
      })

      return new Response(body, {
        status: 200,
        headers: {
          'Content-Type': 'application/vnd.caduceus.rfc822-frames; version=1',
          'Cache-Control': 'no-store',
          'X-Content-Type-Options': 'nosniff',
          'X-Caduceus-Export-Format': 'rfc822-source-frames-v1',
        },
      })
    })
  }

  /** @param {unknown} value */
  static _folder(value) {
    if (typeof value !== 'string' || !value.trim()) return new Error('folder is required')
    const folder = value.trim().toLowerCase()
    if (folder.length > 60 || /[\u0000-\u001f\u007f/\\]/.test(folder)) {
      return new Error('folder is invalid')
    }
    return folder
  }

  /** @param {unknown} value */
  static _providerFolder(value) {
    if (typeof value !== 'string' || !value.trim()) return new Error('folder is required')
    const folder = value.trim()
    if (folder.length > 1024 || /[\u0000-\u001f\u007f]/.test(folder)) return new Error('folder is invalid')
    return folder
  }

  static async *_hostedRecords(app, access, requestedDomain, mappedFolder, ownerAddress) {
    const emails = app.emails.iterateFolder([requestedDomain], mappedFolder, ownerAddress)
    try {
      for (;;) {
        const next = await FyloAccessContext.run(access, () => emails.next())
        if (next.done) return
        const email = next.value
        const source = await FyloAccessContext.run(access, async () => {
          const record = await app.rawMessages.findForEmail(email.id)
          if (!record || record.domain !== requestedDomain) throw new Error(`RFC 822 source is unavailable for message ${email.id}`)
          return app.rawMessages.readContent(record)
        })
        yield { email, source }
      }
    } finally {
      await emails.return?.(undefined)
    }
  }

  static async *_providerRecords(token, accountId, folderStableId) {
    for (let offset = 0; offset <= 100_000; offset += 100) {
      const response = await ExternalAccounts.unifiedMessages(token, { accountId, folderStableId, limit: 100, offset })
      const messages = Array.isArray(response?.messages) ? response.messages : []
      for (const message of messages) {
        const record = await ExternalAccounts.messageSource(token, accountId, String(message.stableId || ''))
        const source = Buffer.from(String(record?.rawRFC822Base64 || ''), 'base64')
        if (!source.byteLength) throw new Error(`RFC 822 source is unavailable for message ${message.stableId}`)
        yield { email: { id: message.stableId, sender: message.sender, subject: message.subject, receivedAt: message.receivedAt }, source }
      }
      if (messages.length < 100) return
    }
    throw new Error('Connected mailbox export exceeds the pagination limit')
  }
}
