// Builds CADUCEUS providers outside Tachyon: for the test suites and for the
// operator scripts that run under Bun. Tachyon fills each empty @Needs field
// with the provider of the same lowerCamel name; this does the same, so tests
// and scripts exercise the classes the server runs.
import { readdirSync } from 'node:fs'
import { join, resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import '../../tests/support/tachyon-markers.mjs'
import { Fylo } from '../../tests/support/legacy-server/shared/persistence/Fylo.js'

export class Providers {
  static ROOT = resolve(import.meta.dirname, '..', '..')

  /** The former App container's field names and the provider field each became. */
  static APP_FIELDS = {
    attachments: 'attachmentsHelper',
    calendarEvents: 'calendarEventsHelper',
    calendarImport: 'calendarImportService',
    categoryMappings: 'categoryMappingService',
    contactRecords: 'contactsHelper',
    contacts: 'contactsService',
    conversationArchive: 'conversationArchiveService',
    domainMigration: 'domainMigrationService',
    domainMigrations: 'domainMigrationsHelper',
    domains: 'domainsHelper',
    drafts: 'draftService',
    emails: 'emailsHelper',
    emlImport: 'emlImportService',
    extensions: 'extensionsHelper',
    followUpReminders: 'followUpRemindersHelper',
    githubProfiles: 'gitHubProfilesService',
    grammarCheck: 'grammarCheckService',
    inbound: 'inboundService',
    inboundDelivery: 'inboundDeliveryService',
    mailActionRecords: 'mailActionsHelper',
    mailActions: 'mailActionService',
    mcpAccess: 'mcpAccessService',
    mcpAccessTokens: 'mcpAccessTokensHelper',
    mcpAuditEvents: 'mcpAuditEventsHelper',
    outboundQueue: 'outboundQueueService',
    outbox: 'outboxHelper',
    presence: 'presenceService',
    providerCalendars: 'providerCalendarService',
    push: 'pushService',
    pushSubscriptions: 'pushSubscriptionsHelper',
    rateLimit: 'rateLimitService',
    rawMessages: 'rawMessagesHelper',
    reminders: 'followUpReminderService',
    rules: 'rulesHelper',
    scheduled: 'scheduledHelper',
    sendIdempotency: 'sendIdempotencyService',
    sendRequestRecords: 'sendRequestsHelper',
    sendService: 'sendService',
    signatures: 'signaturesHelper',
    smtp: 'smtpService',
    snooze: 'snoozeHelper',
    snoozeService: 'snoozeService',
    spamFilter: 'spamFilterService',
    suppressed: 'suppressedHelper',
    templates: 'templatesHelper',
    threadShareRecords: 'threadSharesHelper',
    threadSharing: 'threadSharingService',
    threads: 'threadsHelper',
    tracking: 'trackingHelper',
    users: 'usersHelper',
  }

  /**
   * Former constructor parameters, in order, as provider fields. `...` copies
   * an options object onto the instance.
   */
  static PARAMETERS = {
    AttachmentsHelper: ['fyloHelper'],
    CalendarEventsHelper: ['fyloHelper'],
    CalendarImportService: ['calendarEventsHelper'],
    CategoryMappingService: ['usersHelper', 'emailsHelper'],
    ContactsHelper: ['fyloHelper'],
    ContactsService: ['contactsHelper', 'usersHelper'],
    ConversationArchiveService: ['emailsHelper', 'threadsHelper', 'categoryMappingService'],
    DomainMigrationService: ['domainMigrationsHelper'],
    DomainMigrationsHelper: ['fyloHelper'],
    DomainsHelper: ['fyloHelper'],
    DraftService: ['emailsHelper', 'attachmentsHelper', 'categoryMappingService'],
    EmailsHelper: ['fyloHelper', 'attachmentsHelper', 'rawMessagesHelper'],
    EmlImportService: ['emailsHelper', 'attachmentsHelper', 'rawMessagesHelper', 'inboundService'],
    ExtensionsHelper: ['fyloHelper'],
    FollowUpReminderService: ['followUpRemindersHelper', 'emailsHelper', 'categoryMappingService'],
    FollowUpRemindersHelper: ['fyloHelper'],
    GitHubProfilesService: ['...'],
    InboundDeliveryService: ['attachmentsHelper', 'domainsHelper', 'emailsHelper', 'rawMessagesHelper', 'inboundService', 'pushService', 'calendarImportService', 'spamFilterService', 'followUpReminderService', 'contactsService', 'usersHelper', 'categoryMappingService'],
    InboundService: ['emailsHelper', 'rulesHelper', 'suppressedHelper', 'attachmentsHelper', 'smtpService', 'fyloHelper', 'rawMessagesHelper', 'categoryMappingService'],
    MailActionService: ['mailActionsHelper', 'emailsHelper', 'categoryMappingService'],
    MailActionsHelper: ['fyloHelper'],
    McpAccessService: ['mcpAccessTokensHelper', 'mcpAuditEventsHelper', 'usersHelper'],
    McpAccessTokensHelper: ['fyloHelper'],
    McpAuditEventsHelper: ['fyloHelper'],
    OutboundQueueService: ['outboxHelper', 'smtpService', 'emailsHelper', 'scheduledHelper', 'attachmentsHelper', 'categoryMappingService', 'suppressedHelper'],
    OutboxHelper: ['fyloHelper'],
    PresenceService: ['fyloHelper'],
    ProviderCalendarService: ['boundary'],
    PushService: ['pushSubscriptionsHelper', 'usersHelper', 'emailsHelper'],
    PushSubscriptionsHelper: ['fyloHelper'],
    RateLimitService: ['fyloHelper'],
    RawMessagesHelper: ['fyloHelper'],
    RulesHelper: ['fyloHelper'],
    ScheduledHelper: ['fyloHelper'],
    SendIdempotencyService: ['sendRequestsHelper'],
    SendRequestsHelper: ['fyloHelper'],
    SendService: ['emailsHelper', 'attachmentsHelper', 'rawMessagesHelper', 'scheduledHelper', 'outboxHelper', 'outboundQueueService', 'smtpService', 'contactsService', 'categoryMappingService'],
    SignaturesHelper: ['fyloHelper'],
    SnoozeHelper: ['fyloHelper'],
    SnoozeService: ['snoozeHelper', 'emailsHelper', 'threadsHelper', 'pushService', 'categoryMappingService', 'usersHelper'],
    SpamFilterService: ['...'],
    SuppressedHelper: ['fyloHelper'],
    TemplatesHelper: ['fyloHelper'],
    ThreadSharesHelper: ['fyloHelper'],
    ThreadSharingService: ['threadSharesHelper', 'threadsHelper', 'emailsHelper', 'attachmentsHelper'],
    ThreadsHelper: ['emailsHelper'],
    TrackingHelper: ['fyloHelper'],
    UsersHelper: ['fyloHelper'],
  }

  /**
   * Property names the classes used before their dependencies became
   * provider fields. Tests still reach a dependency by the old name; each
   * built instance answers it with the field.
   */
  static FORMER_PROPERTIES = {
    AttachmentsHelper: { fylo: 'fyloHelper' },
    CalendarEventsHelper: { fylo: 'fyloHelper' },
    CalendarImportService: { calendarEvents: 'calendarEventsHelper' },
    CategoryMappingService: { emails: 'emailsHelper', users: 'usersHelper' },
    ContactsHelper: { fylo: 'fyloHelper' },
    ContactsService: { repository: 'contactsHelper', users: 'usersHelper' },
    ConversationArchiveService: { categoryMappings: 'categoryMappingService', emails: 'emailsHelper', threads: 'threadsHelper' },
    DomainMigrationService: { domainMigrations: 'domainMigrationsHelper' },
    DomainMigrationsHelper: { fylo: 'fyloHelper' },
    DomainsHelper: { fylo: 'fyloHelper' },
    DraftService: { attachments: 'attachmentsHelper', categoryMappings: 'categoryMappingService', emails: 'emailsHelper' },
    EmailsHelper: { attachments: 'attachmentsHelper', fylo: 'fyloHelper', rawMessages: 'rawMessagesHelper' },
    EmlImportService: { attachments: 'attachmentsHelper', emails: 'emailsHelper', inbound: 'inboundService', rawMessages: 'rawMessagesHelper' },
    ExtensionsHelper: { fylo: 'fyloHelper' },
    FollowUpReminderService: { categoryMappings: 'categoryMappingService', emails: 'emailsHelper', repository: 'followUpRemindersHelper' },
    FollowUpRemindersHelper: { fylo: 'fyloHelper' },
    InboundDeliveryService: { attachments: 'attachmentsHelper', calendarImport: 'calendarImportService', categoryMappings: 'categoryMappingService', contacts: 'contactsService', domains: 'domainsHelper', emails: 'emailsHelper', inbound: 'inboundService', push: 'pushService', rawMessages: 'rawMessagesHelper', reminders: 'followUpReminderService', spamFilter: 'spamFilterService', users: 'usersHelper' },
    InboundService: { attachments: 'attachmentsHelper', categoryMappings: 'categoryMappingService', emails: 'emailsHelper', fylo: 'fyloHelper', rawMessages: 'rawMessagesHelper', rules: 'rulesHelper', smtp: 'smtpService', suppressed: 'suppressedHelper' },
    MailActionService: { actions: 'mailActionsHelper', categoryMappings: 'categoryMappingService', emails: 'emailsHelper' },
    MailActionsHelper: { fylo: 'fyloHelper' },
    McpAccessService: { auditEvents: 'mcpAuditEventsHelper', tokens: 'mcpAccessTokensHelper', users: 'usersHelper' },
    McpAccessTokensHelper: { fylo: 'fyloHelper' },
    McpAuditEventsHelper: { fylo: 'fyloHelper' },
    OutboundQueueService: { attachments: 'attachmentsHelper', categoryMappings: 'categoryMappingService', emails: 'emailsHelper', outbox: 'outboxHelper', scheduled: 'scheduledHelper', smtp: 'smtpService', suppressed: 'suppressedHelper' },
    OutboxHelper: { fylo: 'fyloHelper' },
    PresenceService: { fylo: 'fyloHelper' },
    PushService: { emails: 'emailsHelper', pushSubscriptions: 'pushSubscriptionsHelper', users: 'usersHelper' },
    PushSubscriptionsHelper: { fylo: 'fyloHelper' },
    RateLimitService: { fylo: 'fyloHelper' },
    RawMessagesHelper: { fylo: 'fyloHelper' },
    RulesHelper: { fylo: 'fyloHelper' },
    ScheduledHelper: { fylo: 'fyloHelper' },
    SendIdempotencyService: { records: 'sendRequestsHelper' },
    SendRequestsHelper: { fylo: 'fyloHelper' },
    SendService: { attachments: 'attachmentsHelper', categoryMappings: 'categoryMappingService', contacts: 'contactsService', emails: 'emailsHelper', outboundQueue: 'outboundQueueService', outbox: 'outboxHelper', rawMessages: 'rawMessagesHelper', scheduled: 'scheduledHelper', smtp: 'smtpService' },
    SignaturesHelper: { fylo: 'fyloHelper' },
    SnoozeHelper: { fylo: 'fyloHelper' },
    SnoozeService: { categoryMappings: 'categoryMappingService', emails: 'emailsHelper', push: 'pushService', snoozes: 'snoozeHelper', threads: 'threadsHelper', users: 'usersHelper' },
    SuppressedHelper: { fylo: 'fyloHelper' },
    TemplatesHelper: { fylo: 'fyloHelper' },
    ThreadSharesHelper: { fylo: 'fyloHelper' },
    ThreadSharingService: { attachments: 'attachmentsHelper', emails: 'emailsHelper', shares: 'threadSharesHelper', threads: 'threadsHelper' },
    ThreadsHelper: { emails: 'emailsHelper' },
    TrackingHelper: { fylo: 'fyloHelper' },
    UsersHelper: { fylo: 'fyloHelper' },
  }

  /** @type {Map<string, any> | null} */
  static #classes = null

  /** @returns {Promise<Map<string, any>>} provider field name -> class */
  static async classes() {
    if (Providers.#classes) return Providers.#classes
    const classes = new Map()
    for (const folder of ['services', 'helpers']) {
      const directory = join(Providers.ROOT, 'tests', 'support', 'legacy-server', folder)
      for (const file of readdirSync(directory).filter(name => name.endsWith('.js')).sort()) {
        const type = (await import(pathToFileURL(join(directory, file)).href)).default
        classes.set(type.name[0].toLowerCase() + type.name.slice(1), type)
      }
    }
    Providers.#classes = classes
    return classes
  }

  /**
   * Build one provider from the arguments its former constructor took.
   *
   * @param {any} Type
   * @param {...any} args
   * @returns {any}
   */
  static create(Type, ...args) {
    const instance = Providers.instance(Type)
    const names = Providers.PARAMETERS[Type.name] ?? []
    names.forEach((name, index) => {
      if (args[index] === undefined) return
      if (name === '...') Object.assign(instance, args[index])
      else instance[name] = name === 'fyloHelper' ? Providers.fyloHelper(args[index]) : args[index]
    })
    return instance
  }

  /**
   * A new provider that also answers its former property names.
   *
   * @param {any} Type
   */
  static instance(Type) {
    const instance = new Type()
    for (const [former, field] of Object.entries(Providers.FORMER_PROPERTIES[Type.name] ?? {})) {
      Object.defineProperty(instance, former, {
        get: () => instance[field],
        set: value => { instance[field] = field === 'fyloHelper' ? Providers.fyloHelper(value) : value },
        enumerable: false,
        configurable: true,
      })
    }
    return instance
  }

  /**
   * A FyloHelper for an already open database, so the helpers built here
   * share it.
   *
   * @param {any} fylo
   */
  static fyloHelper(fylo) {
    if (fylo && typeof fylo.database === 'function') return fylo
    return { database: async () => fylo }
  }

  /**
   * A container that builds each provider once and fills every empty field
   * named after a provider, like one Tachyon invocation. It also answers the
   * former App field names (`app.emails`) so suites keep their vocabulary.
   *
   * @param {string} [root]
   * @returns {Promise<ProviderContainer>}
   */
  static async container(root) {
    const fylo = await Fylo.createDb(root)
    return new ProviderContainer(fylo, await Providers.classes())
  }
}

export class ProviderContainer {
  /** @type {Map<string, any>} */
  #instances = new Map()

  /** @param {any} fylo @param {Map<string, any>} classes */
  constructor(fylo, classes) {
    this.fylo = fylo
    this.classes = classes
    this.#instances.set('fyloHelper', Providers.fyloHelper(fylo))
    for (const field of [...classes.keys(), 'fyloHelper']) {
      Object.defineProperty(this, field, { get: () => this.resolve(field), set: value => this.provide(field, value), enumerable: false, configurable: true })
    }
    for (const [old, field] of Object.entries(Providers.APP_FIELDS)) {
      Object.defineProperty(this, old, { get: () => this.resolve(field), set: value => this.provide(field, value), enumerable: false, configurable: true })
    }
  }

  /** @param {string} field */
  resolve(field) {
    if (this.#instances.has(field)) return this.#instances.get(field)
    const Type = this.classes.get(field)
    if (!Type) throw new Error(`No provider for field ${field}`)
    const instance = Providers.instance(Type)
    this.#instances.set(field, instance)
    return this.inject(instance)
  }

  /**
   * Replace one provider, for a fake.
   *
   * @param {string} field
   * @param {any} value
   */
  provide(field, value) {
    this.#instances.set(field, value)
    return value
  }

  /**
   * Fill the empty provider fields of an object, such as a controller.
   *
   * @template T
   * @param {T} target
   * @returns {T}
   */
  inject(target) {
    const object = /** @type {any} */ (target)
    for (const key of Object.keys(object)) {
      if (object[key] === undefined && (this.classes.has(key) || key === 'fyloHelper')) object[key] = this.resolve(key)
    }
    return target
  }

  async close() {
    await this.fylo.close()
  }
}
