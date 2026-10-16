export {}

declare global {
  type AttachmentsHelper = import("./helpers/AttachmentsHelper.js").default
  const AttachmentsHelper: typeof import("./helpers/AttachmentsHelper.js").default
  type EmailsHelper = import("./helpers/EmailsHelper.js").default
  const EmailsHelper: typeof import("./helpers/EmailsHelper.js").default
  type FyloHelper = import("./helpers/FyloHelper.js").default
  const FyloHelper: typeof import("./helpers/FyloHelper.js").default
  type OutboxHelper = import("./helpers/OutboxHelper.js").default
  const OutboxHelper: typeof import("./helpers/OutboxHelper.js").default
  type ScheduledHelper = import("./helpers/ScheduledHelper.js").default
  const ScheduledHelper: typeof import("./helpers/ScheduledHelper.js").default
  type SuppressedHelper = import("./helpers/SuppressedHelper.js").default
  const SuppressedHelper: typeof import("./helpers/SuppressedHelper.js").default
  type CategoryMappingService = import("./services/CategoryMappingService.js").default
  const CategoryMappingService: typeof import("./services/CategoryMappingService.js").default
  type SmtpService = import("./services/SmtpService.js").default
  const SmtpService: typeof import("./services/SmtpService.js").default
  type DomainConfig = import("./types").DomainConfig
  type InboxRule = import("./types").InboxRule
  type User = import("./types").User
  type SendRequest = import("./types").SendRequest
  type OutboxEntry = import("./models/OutboxEntry.js").OutboxEntry
  type SmtpAdapter = import("./models/SmtpAdapter.js").SmtpAdapter
  type UsersHelper = import("./helpers/UsersHelper.js").default
  const UsersHelper: typeof import("./helpers/UsersHelper.js").default
  type EmailAttachmentRecord = import("./types").EmailAttachmentRecord
  type EmailAttachmentSummary = import("./types").EmailAttachmentSummary
  type EmailListFilters = import("./models/EmailListFilters.js").EmailListFilters
  type ScheduledEntry = import("./models/ScheduledEntry.js").ScheduledEntry
  type ParsedAttachment = import("./models/ParsedAttachment.js").ParsedAttachment
  type StoredEmail = import("./types").StoredEmail
}
