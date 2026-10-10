import { randomBytes } from 'node:crypto'
import { FyloAccessContext } from '../../../platform/persistence/FyloAccessContext.js'
import { PhishingDetection } from './PhishingDetection.js'
import { ListUnsubscribe } from './ListUnsubscribe.js'
import { threadMetadataFromHeaders } from '../../mailbox/services/threading.js'
import { recipientsFromHeaders } from '../../mailbox/services/personal-level.js'
/** @import { AttachmentsRepository } from '../../mailbox/repositories/AttachmentsRepository.js' */
/** @import { DomainsRepository } from '../../domains/repositories/DomainsRepository.js' */
/** @import { EmailsRepository } from '../../mailbox/repositories/EmailsRepository.js' */
/** @import { RawMessagesRepository } from '../../mailbox/repositories/RawMessagesRepository.js' */
/** @import { InboundService } from './InboundService.js' */
/** @import { PushService } from '../../notifications/services/PushService.js' */
/** @import { CalendarImportService } from '../../calendar/services/CalendarImportService.js' */
/** @import { SpamFilterService } from './SpamFilterService.js' */
/** @import { FollowUpReminderService } from './FollowUpReminderService.js' */
/** @import { ContactsService } from '../../contacts/services/ContactsService.js' */
/** @import { UsersRepository } from '../../users/repositories/UsersRepository.js' */
/** @import { CategoryMappingService } from '../../mailbox/services/CategoryMappingService.js' */

/**
 * Orchestrates the full inbound delivery pipeline: domain check → route
 * match → persist email + attachments → apply post-delivery rules → push
 * notification → calendar auto-import.
 *
 * Returns a `{ status, body }` envelope ready for the route handler to relay.
 */
export class InboundDeliveryService {
  /**
   * @param {AttachmentsRepository} attachments
   * @param {DomainsRepository} domains
   * @param {EmailsRepository} emails
   * @param {RawMessagesRepository} rawMessages
   * @param {InboundService} inbound
   * @param {PushService} push
   * @param {CalendarImportService} calendarImport
   * @param {SpamFilterService} spamFilter
   * @param {FollowUpReminderService} reminders
   * @param {ContactsService} contacts
   * @param {UsersRepository} users
   * @param {CategoryMappingService} categoryMappings
   */
  constructor(attachments, domains, emails, rawMessages, inbound, push, calendarImport, spamFilter, reminders, contacts, users, categoryMappings) {
    this.attachments = attachments
    this.domains = domains
    this.emails = emails
    this.rawMessages = rawMessages
    this.inbound = inbound
    this.push = push
    this.calendarImport = calendarImport
    this.spamFilter = spamFilter
    this.reminders = reminders
    this.contacts = contacts
    this.users = users
    this.categoryMappings = categoryMappings
  }

  /**
   * @param {{ recipient: string, sender: string, subject?: string, body?: string, messageId?: string, spamVerdict?: string, spamScore?: number|string }} msg
   *   `spamVerdict` / `spamScore` are optional structured signals from the
   *   local SMTP receiver; `X-Spam-*` MIME headers are also honored.
   */
  async deliver(msg) {
    if (!msg?.recipient || !msg.sender) {
      return { status: 400, body: { error: 'recipient and sender required' } }
    }

    const recipient = String(msg.recipient).toLowerCase()
    const sender = String(msg.sender).toLowerCase()
    const subject = msg.subject ?? '(no subject)'
    const messageId = msg.messageId ?? randomBytes(16).toString('hex')
    const parsedMessage = await this.inbound.parseMessage(msg.body ?? '')
    const threadMetadata = threadMetadataFromHeaders(parsedMessage.headers, messageId, domainFromAddress(recipient))
    const messageRecipients = recipientsFromHeaders(parsedMessage.headers, recipient)
    const senderSecurity = PhishingDetection.analyze({
      envelopeSender: sender,
      headers: parsedMessage.headers,
    })
    const unsubscribe = ListUnsubscribe.parse(parsedMessage.headers, parsedMessage.html)

    const domain = recipient.split('@')[1]
    if (!domain) return { status: 400, body: { error: 'Invalid recipient address' } }

    const [, domainConfig] = await this.domains.findEntry(domain)
    if (!domainConfig || !domainConfig.inboundEnabled) {
      return { status: 200, body: { accepted: false, reason: 'domain not configured' } }
    }
    const [, mailboxUser] = await this.users.findByEmail(recipient)
    const ownerEmail = mailboxUser?.email ?? recipient
    const inboxFolder = await this.categoryMappings?.pathFor(ownerEmail, domain, 'inbox') || 'inbox'
    const spamFolder = await this.categoryMappings?.pathFor(ownerEmail, domain, 'spam') || 'spam'

    const rule = this.inbound.matchRoute(domainConfig.routes, recipient)
    if (!rule || rule.action.type === 'drop') {
      return { status: 200, body: { accepted: false, reason: 'dropped by route rule' } }
    }

    // Spam classification: route quarantined mail straight to the spam folder
    // and suppress the new-mail notification + calendar auto-import for it.
    const spam = this.spamFilter
      ? this.spamFilter.classify({ sender, subject, text: parsedMessage.text, headers: parsedMessage.headers, verdict: msg.spamVerdict, score: msg.spamScore })
      : { isSpam: false, score: 0, reasons: [] }

    return await FyloAccessContext.runForDomain(domain, async () => {
      try {
        await this.rawMessages.saveForEmail(messageId, domain, msg.body ?? '')
        await this.attachments.saveForEmail(messageId, domain, parsedMessage.attachments)
        await this.emails.put({
          id: messageId,
          domain,
          recipient,
          ownerEmail,
          sender,
          ...(senderSecurity.headerFrom ? { headerFrom: senderSecurity.headerFrom } : {}),
          ...(senderSecurity.replyTo.length ? { replyTo: senderSecurity.replyTo } : {}),
          ...(senderSecurity.securityWarnings.length
            ? { securityWarnings: senderSecurity.securityWarnings }
            : {}),
          ...(unsubscribe.unsubscribeUrl ? { unsubscribeUrl: unsubscribe.unsubscribeUrl } : {}),
          ...(unsubscribe.unsubscribeMailto ? { unsubscribeMailto: unsubscribe.unsubscribeMailto } : {}),
          ...(unsubscribe.unsubscribeBodyUrl ? { unsubscribeBodyUrl: unsubscribe.unsubscribeBodyUrl } : {}),
          ...(unsubscribe.unsubscribeOneClick ? { unsubscribeOneClick: true } : {}),
          ...(messageRecipients.to.length ? { to: messageRecipients.to } : {}),
          ...(messageRecipients.cc.length ? { cc: messageRecipients.cc } : {}),
          ...(messageRecipients.bcc.length ? { bcc: messageRecipients.bcc } : {}),
          ...threadMetadata,
          subject,
          body: parsedMessage.text,
          ...(parsedMessage.html ? { htmlBody: parsedMessage.html } : {}),
          folder: spam.isSpam ? spamFolder : inboxFolder,
          read: false,
          starred: false,
          receivedAt: new Date().toISOString(),
          processed: false,
          ...(spam.score ? { spamScore: spam.score } : {}),
          ...(spam.reasons?.length ? { spamReasons: spam.reasons } : {}),
        })
      } catch (error) {
        await this.attachments.deleteForEmail(messageId).catch(() => {})
        await this.rawMessages.deleteForEmail(messageId).catch(() => {})
        throw error
      }
      await this.inbound.applyRouteAction(rule, { messageId, sender, recipient, subject })
      // User inbox rules only run on legitimately-foldered mail — a user filter
      // shouldn't pull a quarantined message back out of spam.
      if (!spam.isSpam) {
        await this.inbound.applyInboxRules(messageId, ownerEmail, domain, {
          sender,
          recipient,
          to: messageRecipients.to,
          cc: messageRecipients.cc,
          subject,
          body: parsedMessage.text,
          hasAttachment: parsedMessage.attachments.length > 0,
        })

        const [, deliveredEmail] = await this.emails.findById(messageId)
        if (deliveredEmail) await this.push.notifyNewEmail(deliveredEmail)

        // Calendar auto-import is best-effort; never blocks delivery.
        await this.calendarImport.importFromInboundEmail({
          recipient,
          sourceEmailId: messageId,
          textBody: parsedMessage.text,
          attachments: parsedMessage.attachments,
        })

        // A real reply clears any pending "remind me if nobody replies"
        // marker before it can resurface the sent conversation.
        await this.reminders?.resolveForReply({ recipient: ownerEmail, sender, subject, threadId: threadMetadata.threadId })
        await this.contacts?.observeParticipants(ownerEmail, [sender], domain)
      }

      return { status: 200, body: { accepted: true, emailId: messageId, ...(spam.isSpam ? { spam: true, spamScore: spam.score } : {}) } }
    })
  }
}

/** @param {string} address */
function domainFromAddress(address) {
  return address.split('@')[1] || 'localhost'
}
