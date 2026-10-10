import { PhishingDetection } from '../shared/delivery/PhishingDetection.js'
import { ListUnsubscribe } from '../shared/delivery/ListUnsubscribe.js'
import { Threading } from '../shared/mailbox/Threading.js'
import { EmlImportRules } from '../shared/mailbox/EmlImportRules.js'
import { Security } from '../shared/security/Security.js'

/**
 * Import an RFC 822 message into one authorized mailbox without running the
 * SMTP delivery pipeline. File import must not execute domain routes, inbox
 * rules, forwarding, webhooks, push notifications, or reminder resolution.
 */
@Service
export default class EmlImportService {
  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {RawMessagesHelper} */
  @Needs
  rawMessagesHelper

  /** @type {InboundService} */
  @Needs
  inboundService

  /**
   * Persist one exact EML source and its derived searchable representation.
   * File and attachment records are removed if the message row cannot commit.
   *
   * @param {{ source: Uint8Array, domain: string, mailbox: string, folder: string }} input
   */
  async importOne({ source, domain, mailbox, folder }) {
    const parsed = await this.inboundService.parseMessage(source)
    const id = `import-${Security.randomText(16)}`
    const headerFrom = PhishingDetection.extractAddresses(parsed.headers.from)[0]
    const sender = headerFrom || `unknown@${domain}`
    const senderSecurity = PhishingDetection.analyze({ envelopeSender: sender, headers: parsed.headers })
    const unsubscribe = ListUnsubscribe.parse(parsed.headers, parsed.html)
    const to = PhishingDetection.extractAddresses(parsed.headers.to)
    const cc = PhishingDetection.extractAddresses(parsed.headers.cc)
    const receivedAt = EmlImportRules.validDate(parsed.headers.date) || new Date().toISOString()
    const subject = EmlImportRules.safeSubject(parsed.headers.subject)

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
      ...Threading.threadMetadataFromHeaders(parsed.headers, id, domain),
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
      await this.rawMessagesHelper.saveForEmail(id, domain, source)
      await this.attachmentsHelper.replaceForEmail(id, domain, parsed.attachments)
      await this.emailsHelper.put(/** @type {import('../types').StoredEmail} */ (email))
    } catch (error) {
      await this.attachmentsHelper.deleteForEmail(id).catch(() => {})
      await this.rawMessagesHelper.deleteForEmail(id).catch(() => {})
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

