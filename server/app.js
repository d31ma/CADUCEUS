import { createDb } from './shared/persistence/Fylo.js'
import { AttachmentsRepository } from './features/mailbox/repositories/AttachmentsRepository.js'
import { CalendarEventsRepository } from './features/calendar/repositories/CalendarEventsRepository.js'
import { ContactsRepository } from './features/contacts/repositories/ContactsRepository.js'
import { DomainMigrationsRepository } from './features/domains/repositories/DomainMigrationsRepository.js'
import { DomainsRepository } from './features/domains/repositories/DomainsRepository.js'
import { EmailsRepository } from './features/mailbox/repositories/EmailsRepository.js'
import { RawMessagesRepository } from './features/mailbox/repositories/RawMessagesRepository.js'
import { MailActionsRepository } from './features/mailbox/repositories/MailActionsRepository.js'
import { OutboxRepository } from './features/delivery/repositories/OutboxRepository.js'
import { SendRequestsRepository } from './features/delivery/repositories/SendRequestsRepository.js'
import { PushSubscriptionsRepository } from './features/notifications/repositories/PushSubscriptionsRepository.js'
import { RulesRepository } from './features/mailbox/repositories/RulesRepository.js'
import { ScheduledRepository } from './features/delivery/repositories/ScheduledRepository.js'
import { FollowUpRemindersRepository } from './features/delivery/repositories/FollowUpRemindersRepository.js'
import { SnoozeRepository } from './features/mailbox/repositories/SnoozeRepository.js'
import { SuppressedRepository } from './features/delivery/repositories/SuppressedRepository.js'
import { TemplatesRepository } from './features/mailbox/repositories/TemplatesRepository.js'
import { SignaturesRepository } from './features/mailbox/repositories/SignaturesRepository.js'
import { ThreadsRepository } from './features/mailbox/repositories/ThreadsRepository.js'
import { ThreadSharesRepository } from './features/mailbox/repositories/ThreadSharesRepository.js'
import { TrackingRepository } from './features/delivery/repositories/TrackingRepository.js'
import { UsersRepository } from './features/users/repositories/UsersRepository.js'
import { McpAccessTokensRepository } from './features/mcp/repositories/McpAccessTokensRepository.js'
import { McpAuditEventsRepository } from './features/mcp/repositories/McpAuditEventsRepository.js'
import { ExtensionsRepository } from './features/extensions/repositories/ExtensionsRepository.js'

import { CalendarImportService } from './features/calendar/services/CalendarImportService.js'
import { ProviderCalendarService } from './features/calendar/services/ProviderCalendarService.js'
import { ContactsService } from './features/contacts/services/ContactsService.js'
import { GitHubProfilesService } from './features/contacts/services/GitHubProfilesService.js'
import { DomainMigrationService } from './features/domains/services/DomainMigrationService.js'
import { InboundDeliveryService } from './features/delivery/services/InboundDeliveryService.js'
import { OutboundQueueService } from './features/delivery/services/OutboundQueueService.js'
import { SendService } from './features/delivery/services/SendService.js'
import { SendIdempotencyService } from './features/delivery/services/SendIdempotencyService.js'
import { DraftService } from './features/mailbox/services/DraftService.js'
import { GrammarCheckService } from './features/mailbox/services/GrammarCheckService.js'
import { MailActionService } from './features/mailbox/services/MailActionService.js'
import { CategoryMappingService } from './features/mailbox/services/CategoryMappingService.js'
import { ThreadSharingService } from './features/mailbox/services/ThreadSharingService.js'
import { ConversationArchiveService } from './features/mailbox/services/ConversationArchiveService.js'
import { SnoozeService } from './features/mailbox/services/SnoozeService.js'
import { EmlImportService } from './features/mailbox/services/EmlImportService.js'
import { FollowUpReminderService } from './features/delivery/services/FollowUpReminderService.js'
import { SpamFilterService } from './features/delivery/services/SpamFilterService.js'
import { InboundService } from './features/delivery/services/InboundService.js'
import { PresenceService } from './features/presence/services/PresenceService.js'
import { PushService } from './features/notifications/services/PushService.js'
import { RateLimitService } from './platform/rate-limit/RateLimitService.js'
import { SmtpService } from './features/delivery/services/SmtpService.js'
import { McpAccessService } from './features/mcp/services/McpAccessService.js'
import { ServerEnvironment } from './shared/runtime/ServerEnvironment.js'

/**
 * Application container — owns the Fylo instance and every repo and stateful
 * service instance, wired with the correct dependencies.
 *
 * Routes obtain a shared singleton via {@link getApp}. Tests build isolated
 * instances via {@link App.create} pointed at a temp directory.
 */
export class App {
  /** @param {import('./shared/persistence/Fylo.js').Fylo} fylo */
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
    this.providerCalendars = ServerEnvironment.value('FYLO_BRIDGE_ADDRESS') ? new ProviderCalendarService() : null
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
