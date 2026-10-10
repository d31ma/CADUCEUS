import { createDb } from './platform/persistence/fylo.js'
import { AttachmentsRepository } from './features/mailbox/repositories/attachments.js'
import { CalendarEventsRepository } from './features/calendar/repositories/calendar-events.js'
import { ContactsRepository } from './features/contacts/repositories/contacts.js'
import { DomainMigrationsRepository } from './features/domains/repositories/domain-migrations.js'
import { DomainsRepository } from './features/domains/repositories/domains.js'
import { EmailsRepository } from './features/mailbox/repositories/emails.js'
import { RawMessagesRepository } from './features/mailbox/repositories/raw-messages.js'
import { MailActionsRepository } from './features/mailbox/repositories/mail-actions.js'
import { OutboxRepository } from './features/delivery/repositories/outbox.js'
import { SendRequestsRepository } from './features/delivery/repositories/send-requests.js'
import { PushSubscriptionsRepository } from './features/notifications/repositories/push.js'
import { RulesRepository } from './features/mailbox/repositories/rules.js'
import { ScheduledRepository } from './features/delivery/repositories/scheduled.js'
import { FollowUpRemindersRepository } from './features/delivery/repositories/follow-up-reminders.js'
import { SnoozeRepository } from './features/mailbox/repositories/snooze.js'
import { SuppressedRepository } from './features/delivery/repositories/suppressed.js'
import { TemplatesRepository } from './features/mailbox/repositories/templates.js'
import { SignaturesRepository } from './features/mailbox/repositories/signatures.js'
import { ThreadsRepository } from './features/mailbox/repositories/threads.js'
import { ThreadSharesRepository } from './features/mailbox/repositories/thread-shares.js'
import { TrackingRepository } from './features/delivery/repositories/tracking.js'
import { UsersRepository } from './features/users/repositories/users.js'
import { McpAccessTokensRepository } from './features/mcp/repositories/access-tokens.js'
import { McpAuditEventsRepository } from './features/mcp/repositories/audit-events.js'
import { ExtensionsRepository } from './features/extensions/repositories/extensions.js'

import { CalendarImportService } from './features/calendar/services/calendar-import.js'
import { ProviderCalendarService } from './features/calendar/services/provider-calendars.js'
import { ContactsService } from './features/contacts/services/contacts.js'
import { GitHubProfilesService } from './features/contacts/services/github-profiles.js'
import { DomainMigrationService } from './features/domains/services/domain-migration.js'
import { InboundDeliveryService } from './features/delivery/services/inbound-delivery.js'
import { OutboundQueueService } from './features/delivery/services/outbound-queue.js'
import { SendService } from './features/delivery/services/send.js'
import { SendIdempotencyService } from './features/delivery/services/send-idempotency.js'
import { DraftService } from './features/mailbox/services/drafts.js'
import { GrammarCheckService } from './features/mailbox/services/grammar-check.js'
import { MailActionService } from './features/mailbox/services/mail-actions.js'
import { CategoryMappingService } from './features/mailbox/services/category-mappings.js'
import { ThreadSharingService } from './features/mailbox/services/thread-sharing.js'
import { ConversationArchiveService } from './features/mailbox/services/conversation-archive.js'
import { SnoozeService } from './features/mailbox/services/snooze.js'
import { EmlImportService } from './features/mailbox/services/eml-import.js'
import { FollowUpReminderService } from './features/delivery/services/follow-up-reminders.js'
import { SpamFilterService } from './features/delivery/services/spam-filter.js'
import { InboundService } from './features/delivery/services/inbound.js'
import { PresenceService } from './features/presence/services/presence.js'
import { PushService } from './features/notifications/services/push.js'
import { RateLimitService } from './platform/rate-limit/rate-limit.js'
import { SmtpService } from './features/delivery/services/smtp.js'
import { McpAccessService } from './features/mcp/services/access.js'

/**
 * Application container — owns the Fylo instance and every repo and stateful
 * service instance, wired with the correct dependencies.
 *
 * Routes obtain a shared singleton via {@link getApp}. Tests build isolated
 * instances via {@link App.create} pointed at a temp directory.
 */
export class App {
  /** @param {import('./platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo

    // Repositories — Fylo is the only dependency for most; emails and threads
    // need a back-reference to attachments / emails for joins.
    this.attachments       = new AttachmentsRepository(fylo)
    this.calendarEvents    = new CalendarEventsRepository(fylo)
    this.contactRecords    = new ContactsRepository(fylo)
    this.domainMigrations  = new DomainMigrationsRepository(fylo)
    this.domains           = new DomainsRepository(fylo)
    this.rawMessages       = new RawMessagesRepository(fylo)
    this.emails            = new EmailsRepository(fylo, this.attachments, this.rawMessages)
    this.mailActionRecords = new MailActionsRepository(fylo)
    this.outbox            = new OutboxRepository(fylo)
    this.sendRequestRecords = new SendRequestsRepository(fylo)
    this.pushSubscriptions = new PushSubscriptionsRepository(fylo)
    this.rules             = new RulesRepository(fylo)
    this.scheduled         = new ScheduledRepository(fylo)
    this.followUpReminders = new FollowUpRemindersRepository(fylo)
    this.snooze            = new SnoozeRepository(fylo)
    this.suppressed        = new SuppressedRepository(fylo)
    this.templates         = new TemplatesRepository(fylo)
    this.signatures        = new SignaturesRepository(fylo)
    this.threads           = new ThreadsRepository(this.emails)
    this.threadShareRecords = new ThreadSharesRepository(fylo)
    this.tracking          = new TrackingRepository(fylo)
    this.users             = new UsersRepository(fylo)
    this.mcpAccessTokens   = new McpAccessTokensRepository(fylo)
    this.mcpAuditEvents    = new McpAuditEventsRepository(fylo)
    this.extensions        = new ExtensionsRepository(fylo)
    this.categoryMappings  = new CategoryMappingService(this.users, this.emails)
    this.mailActions       = new MailActionService(this.mailActionRecords, this.emails, this.categoryMappings)
    this.mcpAccess         = new McpAccessService(this.mcpAccessTokens, this.mcpAuditEvents, this.users)

    // Stateless or self-contained services.
    this.smtp      = new SmtpService()
    this.rateLimit = new RateLimitService(fylo)
    this.presence  = new PresenceService(fylo)

    // Services that consume repositories and other services.
    this.push             = new PushService(this.pushSubscriptions, this.users, this.emails)
    this.snoozeService    = new SnoozeService(this.snooze, this.emails, this.threads, this.push, this.categoryMappings, this.users)
    this.calendarImport   = new CalendarImportService(this.calendarEvents)
    this.providerCalendars = process.env.FYLO_BRIDGE_ADDRESS ? new ProviderCalendarService() : null
    this.contacts         = new ContactsService(this.contactRecords, this.users)
    this.githubProfiles   = new GitHubProfilesService()
    this.conversationArchive = new ConversationArchiveService(this.emails, this.threads, this.categoryMappings)
    this.reminders        = new FollowUpReminderService(this.followUpReminders, this.emails, this.categoryMappings)
    this.domainMigration  = new DomainMigrationService(this.domainMigrations)
    this.spamFilter       = new SpamFilterService()
    this.inbound          = new InboundService(this.emails, this.rules, this.suppressed, this.attachments, this.smtp, fylo, this.rawMessages, this.categoryMappings)
    this.emlImport        = new EmlImportService(this.emails, this.attachments, this.rawMessages, this.inbound)
    this.inboundDelivery  = new InboundDeliveryService(
      this.attachments,
      this.domains,
      this.emails,
      this.rawMessages,
      this.inbound,
      this.push,
      this.calendarImport,
      this.spamFilter,
      this.reminders,
      this.contacts,
      this.users,
      this.categoryMappings,
    )
    this.outboundQueue    = new OutboundQueueService(this.outbox, this.smtp, this.emails, this.scheduled, this.attachments, this.categoryMappings, this.suppressed)
    this.sendService      = new SendService(this.emails, this.attachments, this.rawMessages, this.scheduled, this.outbox, this.outboundQueue, this.smtp, this.contacts, this.categoryMappings)
    this.sendIdempotency  = new SendIdempotencyService(this.sendRequestRecords)
    this.drafts           = new DraftService(this.emails, this.attachments, this.categoryMappings)
    this.grammarCheck     = new GrammarCheckService()
    this.threadSharing    = new ThreadSharingService(
      this.threadShareRecords,
      this.threads,
      this.emails,
      this.attachments,
    )
  }

  /**
   * Build an App backed by Fylo rooted at the given directory (or `FYLO_ROOT`).
   * Use this from tests with a temp directory for isolation.
   *
   * @param {string} [root]
   * @returns {Promise<App>}
   */
  static async create(root) {
    const fylo = await createDb(root)
    return new App(fylo)
  }

  /**
   * Release process-owned resources before the data root is removed or handed
   * to another writer.
   *
   * @returns {Promise<void>}
   */
  async close() {
    await this.fylo.close()
  }
}

/** @type {Promise<App> | null} */
let cached = null

/**
 * Lazy singleton for production / serve. Subsequent calls share the same
 * Fylo instance and the same in-memory service state (rate-limit counters,
 * cached SMTP adapter). Tests should call {@link App.create} directly with a
 * temp root and not touch this singleton.
 *
 * @returns {Promise<App>}
 */
export function getApp() {
  if (!cached) cached = App.create()
  return cached
}

/**
 * Drop the cached singleton. Provided for tests that explicitly want to
 * exercise the cold-start path; production code never calls this.
 */
export async function resetApp() {
  const current = cached
  cached = null
  if (!current) return
  const app = await current
  await app.close()
}

/**
 * Install a pre-built App as the singleton so in-process route tests can point
 * handlers at a temp-rooted instance. Production code never calls this.
 *
 * @param {App} app
 */
export function setApp(app) {
  cached = Promise.resolve(app)
}
