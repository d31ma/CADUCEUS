import { randomBytes } from 'node:crypto'
import { PhishingDetection } from '../../delivery/services/PhishingDetection.js'
import { ListUnsubscribe } from '../../delivery/services/ListUnsubscribe.js'
import { threadMetadataFromHeaders } from './threading.js'

const MAX_SUBJECT_LENGTH = 500

/**
 * Import an RFC 822 message into one authorized mailbox without running the
 * SMTP delivery pipeline. File import must not execute domain routes, inbox
 * rules, forwarding, webhooks, push notifications, or reminder resolution.
 */
export class EmlImportService {
  /**
   * @param {import('../repositories/EmailsRepository.js').EmailsRepository} emails
   * @param {import('../repositories/AttachmentsRepository.js').AttachmentsRepository} attachments
   * @param {import('../repositories/RawMessagesRepository.js').RawMessagesRepository} rawMessages
   * @param {import('../../delivery/services/InboundService.js').InboundService} inbound
   */
  constructor(emails, attachments, rawMessages, inbound) {
    this.emails = emails
    this.attachments = attachments
    this.rawMessages = rawMessages
    this.inbound = inbound
  }

  /**
   * Persist one exact EML source and its derived searchable representation.
   * File and attachment records are removed if the message row cannot commit.
   *
   * @param {{ source: Uint8Array, domain: string, mailbox: string, folder: string }} input
   */
  async importOne({ source, domain, mailbox, folder }) {
    const parsed = await this.inbound.parseMessage(source)
    const id = `import-${randomBytes(16).toString('hex')}`
    const headerFrom = PhishingDetection.extractAddresses(parsed.headers.from)[0]
    const sender = headerFrom || `unknown@${domain}`
    const senderSecurity = PhishingDetection.analyze({ envelopeSender: sender, headers: parsed.headers })
    const unsubscribe = ListUnsubscribe.parse(parsed.headers, parsed.html)
    const to = PhishingDetection.extractAddresses(parsed.headers.to)
    const cc = PhishingDetection.extractAddresses(parsed.headers.cc)
    const receivedAt = validDate(parsed.headers.date) || new Date().toISOString()
    const subject = safeSubject(parsed.headers.subject)

    const email = {
      id,
      domain,
      recipient: mailbox,
      ...(to[0] && to[0] !== mailbox ? { originalRecipient: to[0] } : {}),
      sender,
      ...(senderSecurity.headerFrom ? { headerFrom: senderSecurity.headerFrom } : {}),
      ...(senderSecurity.replyTo.length ? { replyTo: senderSecurity.replyTo } : {}),
      ...(senderSecurity.securityWarnings.length ? { securityWarnings: senderSecurity.securityWarnings } : {}),
      ...(unsubscribe.unsubscribeUrl ? { unsubscribeUrl: unsubscribe.unsubscribeUrl } : {}),
      ...(unsubscribe.unsubscribeMailto ? { unsubscribeMailto: unsubscribe.unsubscribeMailto } : {}),
      ...(unsubscribe.unsubscribeBodyUrl ? { unsubscribeBodyUrl: unsubscribe.unsubscribeBodyUrl } : {}),
      ...(unsubscribe.unsubscribeOneClick ? { unsubscribeOneClick: true } : {}),
      ...threadMetadataFromHeaders(parsed.headers, id, domain),
      ...(to.length ? { to } : {}),
      ...(cc.length ? { cc } : {}),
      subject,
      body: parsed.text,
      ...(parsed.html ? { htmlBody: parsed.html } : {}),
      folder,
      read: false,
      starred: false,
      receivedAt,
      processed: true,
      ...(parsed.attachments.length ? { hasAttachments: true } : {}),
    }

    try {
      await this.rawMessages.saveForEmail(id, domain, source)
      await this.attachments.replaceForEmail(id, domain, parsed.attachments)
      await this.emails.put(/** @type {import('../../../types').StoredEmail} */ (email))
    } catch (error) {
      await this.attachments.deleteForEmail(id).catch(() => {})
      await this.rawMessages.deleteForEmail(id).catch(() => {})
      throw error
    }

    return {
      id,
      subject,
      sender,
      receivedAt,
      attachmentCount: parsed.attachments.length,
    }
  }
}

/** @param {unknown} value */
function safeSubject(value) {
  return String(value || '(no subject)')
    .replace(/[\u0000-\u001f\u007f]+/g, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, MAX_SUBJECT_LENGTH) || '(no subject)'
}

/** @param {unknown} value */
function validDate(value) {
  const time = Date.parse(String(value || ''))
  return Number.isFinite(time) ? new Date(time).toISOString() : ''
}
