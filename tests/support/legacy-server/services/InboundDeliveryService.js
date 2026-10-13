import { FyloAccessContext } from '../shared/persistence/FyloAccessContext.js'
import { PhishingDetection } from '../shared/delivery/PhishingDetection.js'
import { ListUnsubscribe } from '../shared/delivery/ListUnsubscribe.js'
import { Threading } from '../shared/mailbox/Threading.js'
import { PersonalLevel } from '../shared/mailbox/PersonalLevel.js'
import { InboundDeliveryRules } from '../shared/delivery/InboundDeliveryRules.js'
import { Security } from '../shared/security/Security.js'

/** @import { AttachmentsHelper } from '../helpers/AttachmentsHelper.js' */
/** @import { DomainsHelper } from '../helpers/DomainsHelper.js' */
/** @import { EmailsHelper } from '../helpers/EmailsHelper.js' */
/** @import { RawMessagesHelper } from '../helpers/RawMessagesHelper.js' */
/** @import { InboundService } from './InboundService.js' */
/** @import { PushService } from './PushService.js' */
/** @import { CalendarImportService } from './CalendarImportService.js' */
/** @import { SpamFilterService } from './SpamFilterService.js' */
/** @import { FollowUpReminderService } from './FollowUpReminderService.js' */
/** @import { ContactsService } from './ContactsService.js' */
/** @import { UsersHelper } from '../helpers/UsersHelper.js' */
/** @import { CategoryMappingService } from './CategoryMappingService.js' */

/**
 * Orchestrates the full inbound delivery pipeline: domain check → route
 * match → persist email + attachments → apply post-delivery rules → push
 * notification → calendar auto-import.
 *
 * Returns a `{ status, body }` envelope ready for the route handler to relay.
 */
@Service
export default class InboundDeliveryService {
  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {DomainsHelper} */
  @Needs
  domainsHelper

  /** @type {EmailsHelper} */
  @Needs
  emailsHelper

  /** @type {RawMessagesHelper} */
  @Needs
  rawMessagesHelper

  /** @type {InboundService} */
  @Needs
  inboundService

  /** @type {PushService} */
  @Needs
  pushService

  /** @type {CalendarImportService} */
  @Needs
  calendarImportService

  /** @type {SpamFilterService} */
  @Needs
  spamFilterService

  /** @type {FollowUpReminderService} */
  @Needs
  followUpReminderService

  /** @type {ContactsService} */
  @Needs
  contactsService

  /** @type {UsersHelper} */
  @Needs
  usersHelper

  /** @type {CategoryMappingService} */
  @Needs
  categoryMappingService

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
    const messageId = msg.messageId ?? Security.randomText(16)
    const parsedMessage = await this.inboundService.parseMessage(msg.body ?? '')
    const threadMetadata = Threading.threadMetadataFromHeaders(parsedMessage.headers, messageId, InboundDeliveryRules.domainFromAddress(recipient))
    const messageRecipients = PersonalLevel.recipientsFromHeaders(parsedMessage.headers, recipient)
    const senderSecurity = PhishingDetection.analyze({
      envelopeSender: sender,
      headers: parsedMessage.headers,
    })
    const unsubscribe = ListUnsubscribe.parse(parsedMessage.headers, parsedMessage.html)

    const domain = recipient.split('@')[1]
    if (!domain) return { status: 400, body: { error: 'Invalid recipient address' } }

    const [, domainConfig] = await this.domainsHelper.findEntry(domain)
    if (!domainConfig || !domainConfig.inboundEnabled) {
      return { status: 200, body: { accepted: false, reason: 'domain not configured' } }
    }
    const [, mailboxUser] = await this.usersHelper.findByEmail(recipient)
    const ownerEmail = mailboxUser?.email ?? recipient
    const inboxFolder = await this.categoryMappingService?.pathFor(ownerEmail, domain, 'inbox') || 'inbox'
    const spamFolder = await this.categoryMappingService?.pathFor(ownerEmail, domain, 'spam') || 'spam'

    const rule = this.inboundService.matchRoute(domainConfig.routes, recipient)
    if (!rule || rule.action.type === 'drop') {
      return { status: 200, body: { accepted: false, reason: 'dropped by route rule' } }
    }

    // Spam classification: route quarantined mail straight to the spam folder
    // and suppress the new-mail notification + calendar auto-import for it.
    const spam = this.spamFilterService
      ? this.spamFilterService.classify({ sender, subject, text: parsedMessage.text, headers: parsedMessage.headers, verdict: msg.spamVerdict, score: msg.spamScore })
      : { isSpam: false, score: 0, reasons: [] }

    return await FyloAccessContext.runForDomain(domain, async () => {
      try {
        await this.rawMessagesHelper.saveForEmail(messageId, domain, msg.body ?? '')
        await this.attachmentsHelper.saveForEmail(messageId, domain, parsedMessage.attachments)
        await this.emailsHelper.put({
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
        await this.attachmentsHelper.deleteForEmail(messageId).catch(() => {})
        await this.rawMessagesHelper.deleteForEmail(messageId).catch(() => {})
        throw error
      }
      await this.inboundService.applyRouteAction(rule, { messageId, sender, recipient, subject })
      // User inbox rules only run on legitimately-foldered mail — a user filter
      // shouldn't pull a quarantined message back out of spam.
      if (!spam.isSpam) {
        await this.inboundService.applyInboxRules(messageId, ownerEmail, domain, {
          sender,
          recipient,
          to: messageRecipients.to,
          cc: messageRecipients.cc,
          subject,
          body: parsedMessage.text,
          hasAttachment: parsedMessage.attachments.length > 0,
        })

        const [, deliveredEmail] = await this.emailsHelper.findById(messageId)
        if (deliveredEmail) await this.pushService.notifyNewEmail(deliveredEmail)

        // Calendar auto-import is best-effort; never blocks delivery.
        await this.calendarImportService.importFromInboundEmail({
          recipient,
          sourceEmailId: messageId,
          textBody: parsedMessage.text,
          attachments: parsedMessage.attachments,
        })

        // A real reply clears any pending "remind me if nobody replies"
        // marker before it can resurface the sent conversation.
        await this.followUpReminderService?.resolveForReply({ recipient: ownerEmail, sender, subject, threadId: threadMetadata.threadId })
        await this.contactsService?.observeParticipants(ownerEmail, [sender], domain)
      }

      return { status: 200, body: { accepted: true, emailId: messageId, ...(spam.isSpam ? { spam: true, spamScore: spam.score } : {}) } }
    })
  }
}

