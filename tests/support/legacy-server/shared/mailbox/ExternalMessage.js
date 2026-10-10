import { Buffer } from 'node:buffer'
import { simpleParser } from '../../deps/npm/mailparser.mjs'
import { Threading } from './Threading.js'

export class ExternalMessage {
  static #ACCOUNT_ID = /^ext_[A-Za-z0-9_-]{20,80}$/

  static #MESSAGE_PREFIX = 'extmsg.'

  static #ATTACHMENT_ID = /^extatt\.(\d{1,3})$/

  /** Create an opaque route-safe identity. Every decoded value is reauthorized by Go. */
  static externalMessageId(accountId, stableId) {
    if (!ExternalMessage.#ACCOUNT_ID.test(accountId) || !ExternalMessage.#validStableId(stableId)) return ''
    return ExternalMessage.#MESSAGE_PREFIX + Buffer.from(JSON.stringify({ accountId, stableId }), 'utf8').toString('base64url')
  }

  static parseExternalMessageId(value) {
    if (typeof value !== 'string' || !value.startsWith(ExternalMessage.#MESSAGE_PREFIX) || value.length > 2048) return null
    try {
      const decoded = JSON.parse(Buffer.from(value.slice(ExternalMessage.#MESSAGE_PREFIX.length), 'base64url').toString('utf8'))
      if (!decoded || typeof decoded !== 'object' || Object.keys(decoded).sort().join(',') !== 'accountId,stableId') return null
      if (!ExternalMessage.#ACCOUNT_ID.test(decoded.accountId) || !ExternalMessage.#validStableId(decoded.stableId)) return null
      return { accountId: decoded.accountId, stableId: decoded.stableId }
    } catch {
      return null
    }
  }

  /** Convert the Go-owned redacted index into the normal inbox row contract. */
  static presentExternalSummary(message) {
    const id = ExternalMessage.externalMessageId(message?.accountId, message?.stableId)
    if (!id) return null
    const flags = new Set((message.flags || []).map(value => String(value).toLowerCase()))
    const folder = String(message.folderRole || message.folderName || 'inbox').toLowerCase()
    const mailboxAddress = String(message.mailboxAddress || '').toLowerCase()
    const domain = mailboxAddress.split('@')[1] || ''
    return {
      id,
      external: true,
      providerAccountId: message.accountId,
      providerStableId: message.stableId,
      mailboxAddress,
      mailboxLabel: message.accountLabel || mailboxAddress,
      mailboxColor: message.accountColor || '#64748b',
      domain,
      sender: message.sender || '',
      recipient: message.to?.[0] || mailboxAddress,
      to: message.to || [],
      cc: message.cc || [],
      bcc: message.bcc || [],
      subject: message.subject || '',
      snippet: message.snippet || '',
      messageId: message.rfcMessageId || '',
      rfcMessageId: message.rfcMessageId || '',
      inReplyTo: message.inReplyTo || '',
      references: message.references || [],
      folder,
      systemFolder: folder,
      read: flags.has('\\seen'),
      starred: flags.has('\\flagged'),
      receivedAt: message.receivedAt,
      rawSize: Number.isSafeInteger(Number(message.size)) ? Number(message.size) : 0,
      hasAttachments: Array.isArray(message.attachmentNames) && message.attachmentNames.length > 0,
      attachmentNames: message.attachmentNames || [],
      labels: Array.isArray(message.labels)
        ? [...new Set(message.labels.map(value => String(value || '').trim()).filter(Boolean))]
        : folder && !['inbox', 'sent', 'drafts', 'spam', 'archive', 'trash'].includes(folder) ? [folder] : [],
    }
  }

  /** Parse an already-authorized provider source into the normal reader contract. */
  static async presentExternalDetail(indexed, rawRFC822Base64) {
    const summary = ExternalMessage.presentExternalSummary(indexed)
    if (!summary || typeof rawRFC822Base64 !== 'string' || rawRFC822Base64.length > 35_000_000) return null
    const raw = Buffer.from(rawRFC822Base64, 'base64')
    if (raw.length < 1 || raw.length > 25 * 1024 * 1024) return null
    const parsed = await simpleParser(raw)
    const headers = ExternalMessage.#flattenHeaders(parsed.headers)
    const attachments = (parsed.attachments || []).slice(0, 100).map((attachment, index) => ({
      id: `extatt.${index}`,
      filename: ExternalMessage.#portableFilename(attachment.filename || 'attachment'),
      contentType: String(attachment.contentType || 'application/octet-stream').slice(0, 255),
      size: Number(attachment.size || attachment.content?.length || 0),
      disposition: attachment.contentDisposition === 'inline' ? 'inline' : 'attachment',
      ...(attachment.contentId ? { contentId: String(attachment.contentId).replace(/^<|>$/g, '').slice(0, 998) } : {}),
    }))
    const replyTo = ExternalMessage.#addressValues(parsed.replyTo)
    return {
      ...summary,
      sender: parsed.from?.text || summary.sender,
      to: ExternalMessage.#addressValues(parsed.to),
      cc: ExternalMessage.#addressValues(parsed.cc),
      bcc: ExternalMessage.#addressValues(parsed.bcc),
      subject: parsed.subject || summary.subject,
      body: parsed.text || '',
      ...(typeof parsed.html === 'string' ? { htmlBody: parsed.html } : {}),
      ...(replyTo.length ? { replyTo } : {}),
      headers,
      ...Threading.threadMetadataFromHeaders(headers, summary.id, summary.domain || 'external.invalid'),
      hasRawSource: true,
      hasAttachments: attachments.length > 0,
      attachments,
    }
  }

  /** Select one attachment only after its containing external message was authorized. */
  static async readExternalAttachment(rawRFC822Base64, attachmentId) {
    const match = typeof attachmentId === 'string' ? attachmentId.match(ExternalMessage.#ATTACHMENT_ID) : null
    if (!match || typeof rawRFC822Base64 !== 'string' || rawRFC822Base64.length > 35_000_000) return null
    const raw = Buffer.from(rawRFC822Base64, 'base64')
    if (raw.length < 1 || raw.length > 25 * 1024 * 1024) return null
    const parsed = await simpleParser(raw)
    const index = Number(match[1])
    const attachment = parsed.attachments?.[index]
    if (!attachment || index >= 100) return null
    const content = Buffer.from(attachment.content || [])
    return {
      id: attachmentId,
      filename: ExternalMessage.#portableFilename(attachment.filename || 'attachment'),
      contentType: String(attachment.contentType || 'application/octet-stream').slice(0, 255),
      size: content.length,
      contentBase64: content.toString('base64'),
    }
  }

  static #validStableId(value) {
    return typeof value === 'string' && value.length >= 1 && value.length <= 512 && !/[\0\r\n]/.test(value)
  }

  static #addressValues(value) {
    const values = Array.isArray(value) ? value : value ? [value] : []
    return [...new Set(values.flatMap(entry => entry?.value || []).map(entry => String(entry?.address || '').toLowerCase()).filter(Boolean))].slice(0, 100)
  }

  static #flattenHeaders(headers) {
    const result = {}
    headers?.forEach?.((value, key) => {
      const addresses = Array.isArray(value?.value) ? value.value.map(entry => entry?.address || '').filter(Boolean).join(', ') : ''
      result[String(key).toLowerCase()] = typeof value === 'string' ? value : addresses || String(value?.value ?? value ?? '')
    })
    return result
  }

  static #portableFilename(value) {
    const leaf = String(value || '').replaceAll('\\', '/').split('/').pop()?.trim() || 'attachment'
    return leaf.replace(/[\0\r\n]/g, '').slice(0, 255) || 'attachment'
  }
}
