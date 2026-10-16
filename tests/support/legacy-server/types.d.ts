// ── Domain & Routing ─────────────────────────────────────────────────────────

export interface DomainConfig {
  domain: string
  routes: RouteRule[]
  inboundEnabled: boolean
}

export interface DomainMigration {
  id: string
  fromDomain: string
  toDomain: string
  mode: 'alias' | 'cutover'
  localPartStrategy: 'preserve'
  createdAt: string
  appliedAt?: string
}

export interface RouteRule {
  id: string
  /** Recipient match: exact address, wildcard prefix (e.g. "*@example.com"), or catchall "*" */
  match: string
  action: RouteAction
  enabled: boolean
}

export type RouteAction =
  | { type: 'webhook'; url: string; secret?: string }
  | { type: 'forward'; to: string }
  | { type: 'store' }
  | { type: 'drop' }

// ── Email Storage ─────────────────────────────────────────────────────────────

export interface StoredEmail {
  id: string
  domain: string
  recipient: string
  originalRecipient?: string
  sender: string
  headerFrom?: string
  replyTo?: string[]
  securityWarnings?: Array<'reply_to_mismatch'>
  unsubscribeUrl?: string
  unsubscribeMailto?: string
  unsubscribeBodyUrl?: string
  unsubscribeOneClick?: boolean
  /** Canonical RFC 5322 Message-ID, distinct from CADUCEUS's storage id. */
  rfcMessageId?: string
  inReplyTo?: string
  references?: string[]
  /** Tenant-scoped conversation identity derived from the RFC reference root. */
  threadId?: string
  snoozeId?: string
  snoozeThreadId?: string
  snoozedUntil?: string
  snoozeLabel?: string
  subject: string
  body: string
  htmlBody?: string
  folder: string
  /** Independent provider-style labels; folder membership remains singular. */
  labels?: string[]
  systemFolder?: 'inbox' | 'sent' | 'drafts' | 'spam' | 'archive' | 'trash' | null
  read: boolean
  starred: boolean
  receivedAt: string
  processed: boolean
  /** Exact authenticated mailbox owner; distinct from a selectable From alias. */
  ownerEmail?: string
  trackingId?: string
  openTracking?: boolean
  linkTracking?: boolean
  trackedLinksJson?: string
  opensJson?: string
  clicksJson?: string
  trackedLinks?: TrackingLink[]
  opens?: TrackingOpenEvent[]
  clicks?: TrackingClickEvent[]
  openedAt?: string
  openCount?: number
  clickCount?: number
  spamScore?: number | null
  spamReasons?: string[] | null
  bounce?: boolean
  to?: string[]
  cc?: string[]
  bcc?: string[]
  draftOwner?: string
  draftClientId?: string
  draftVersion?: number
  draftUpdatedAt?: string
  /** Source message retained by reply/forward drafts for Send & Archive. */
  sourceEmailId?: string
}

export interface TrackingLink {
  index: number
  url: string
  title: string
}

export interface TrackingOpenEvent {
  id: string
  recipient?: string
  openedAt: string
}

export interface TrackingClickEvent {
  id: string
  recipient?: string
  url: string
  title?: string
  linkIndex: number
  clickedAt: string
}

export interface SendAttachment {
  filename: string
  contentType: string
  contentBase64: string
  disposition?: 'inline' | 'attachment'
  contentId?: string
}

export interface EmailAttachmentSummary {
  id: string
  filename: string
  contentType: string
  size: number
  disposition?: string
  contentId?: string
}

export interface EmailAttachmentRecord extends EmailAttachmentSummary {
  emailId: string
  domain: string
  bucketId?: string
  /** Legacy path-backed records remain readable during migration. */
  storagePath?: string
  createdAt: string
}

export interface RawMessageRecord {
  id: string
  emailId: string
  domain: string
  bucketId: string
  size: number
  sha256: string
  createdAt: string
}

export interface ThreadShareRecord {
  id: string
  ownerEmail: string
  domain: string
  threadId: string
  subject: string
  emailIds: string[]
  tokenHash: string
  createdAt: string
  expiresAt: string
  revokedAt?: string
}

export interface McpPreferences {
  enabled: boolean
  accessLevel: 'read-only' | 'read-write' | 'read-write-send'
  allowedDomains: string[]
  excludedFolders: Record<string, string[]>
}

export interface McpAccessTokenRecord {
  id: string
  ownerEmail: string
  domain: string
  tokenHash: string
  active: boolean
  createdAt: string
  revokedAt?: string
  lastUsedAt?: string
}

export interface McpAuditEvent {
  id: string
  ownerEmail: string
  domain: string
  tokenId: string
  toolName: string
  category: 'read' | 'write' | 'send'
  status: 'ok' | 'error' | 'denied'
  parameterKeysJson: string
  resultCount: number
  durationMs: number
  createdAt: string
}

// ── Suppression ───────────────────────────────────────────────────────────────

export interface SuppressedAddress {
  address: string
  reason: 'bounce' | 'complaint'
  suppressedAt: string
}

// ── Users ─────────────────────────────────────────────────────────────────────

export interface User {
  id?: string
  email: string
  aliases?: string[]
  domains: string[]
  role: 'admin' | 'viewer'
  calendarVisibility?: 'private' | 'busy' | 'full'
  mailboxNames?: Record<string, string>
  categoryMappings?: Record<string, Record<'inbox' | 'sent' | 'drafts' | 'spam' | 'archive' | 'trash', string>>
  categoryCatalog?: Record<string, Record<string, 'folder' | 'label'>>
  remoteImagePolicy?: 'block' | 'allow'
  trustedImageSenders?: string[]
  interfaceFont?: 'plex-sans' | 'system' | 'humanist' | 'serif' | 'monospace'
  composerFont?: 'interface' | 'plex-sans' | 'system' | 'humanist' | 'serif' | 'monospace'
  interfaceLocale?: 'auto' | 'en' | 'de' | 'es' | 'es-419' | 'fr' | 'ko' | 'pl' | 'ja' | 'zh-CN' | 'zh-TW' | 'pt' | 'pt-BR' | 'gl' | 'tr' | 'ru' | 'cs' | 'uk' | 'hu' | 'he' | 'ar'
  notificationSounds?: {
    'new-mail': 'none' | 'chime' | 'bell' | 'pop' | 'pulse'
    'hit-send': 'none' | 'chime' | 'bell' | 'pop' | 'pulse'
    send: 'none' | 'chime' | 'bell' | 'pop' | 'pulse'
    confirm: 'none' | 'chime' | 'bell' | 'pop' | 'pulse'
  }
  themePreferences?: {
    activeTheme: 'automatic' | 'light' | 'dark' | 'darkside' | 'less-is-more' | 'taiga' | 'ubuntu'
    lightTheme: 'light' | 'less-is-more' | 'taiga' | 'ubuntu'
    darkTheme: 'dark' | 'darkside'
  }
  githubProfileEnrichment?: boolean
  grammarCheckEnabled?: boolean
  grammarCheckLanguage?: 'auto' | 'en-US' | 'en-GB'
  grammarCheckDisabledRules?: string[]
  unreadBadgeScope?: 'all' | 'domain'
  unreadBadgeDomain?: string
  newMailNotificationDomains?: string[]
  newMailNotificationExternalAccounts?: string[]
  newMailNotificationPrivacy?: 'full' | 'sender' | 'private'
  newMailNotificationActions?: boolean
  keyboardShortcutOverrides?: Record<string, string>
  mailPreferences?: MailPreferences
  mcp?: McpPreferences
}

export interface MailPreferences {
  workspaceLayout: 'split' | 'list' | 'split-vertical'
  use24HourClock: boolean
  interfaceScale: 0.8 | 0.9 | 1 | 1.1 | 1.2
  showUnreadForAllFolders: boolean
  markAsReadDelay: -1 | 0 | 500 | 2000
  detailedHeaders: boolean
  detailedNames: boolean
  restrictMessageWidth: boolean
  backspaceAction: 'archive' | 'trash'
  swipeEnabled: boolean
  conversationOrder: 'ascending' | 'descending'
  emailRenderMode: 'theme' | 'light' | 'dark'
  richText: boolean
  spellcheck: boolean
  spellcheckLanguage: string
  defaultSendAction: 'send' | 'send-and-archive'
  defaultReplyAction: 'reply' | 'reply-all'
  undoSendDelay: 0 | 5000 | 15000 | 30000 | 60000
  defaultFrom: string
  autoCcBySender: Record<string, string[]>
  autoBccBySender: Record<string, string[]>
  displayAttachmentPreviews: boolean
  unsnoozeToTop: boolean
  findInMailDisabledDomains: string[]
}

// ── Contacts ─────────────────────────────────────────────────────────────────

export interface ContactValue {
  value: string
  type?: string
}

export interface ContactAddress {
  streetAddress?: string
  extendedAddress?: string
  city?: string
  region?: string
  postalCode?: string
  country?: string
  type?: string
}

export interface Contact {
  id: string
  ownerEmail: string
  domain: string
  source: 'manual' | 'mail' | 'vcard'
  name: string
  givenName?: string
  familyName?: string
  company?: string
  title?: string
  emails: ContactValue[]
  phones: ContactValue[]
  addresses: ContactAddress[]
  urls: ContactValue[]
  nicknames: string[]
  birthday?: string
  notes?: string
  groupIds: string[]
  interactionCount?: number
  lastContactedAt?: string
  createdAt: string
  updatedAt: string
}

export interface ContactGroup {
  id: string
  ownerEmail: string
  domain: string
  name: string
  createdAt: string
  updatedAt: string
}

// ── Auth Sessions ─────────────────────────────────────────────────────────────

/** Active secondary-verification session created after primary authentication. */
export interface MfaSession {
  id: string
  email: string
  expiresAt: string
  failedAttempts?: number
  primaryAmr?: string[]
}

/** Pending TOTP device registration session */
export interface SetupSession {
  id: string
  email: string
  /** AES-GCM encrypted Base32 TOTP secret */
  totpSecret: string
  expiresAt: string
}

export interface AuthCredential {
  userEmail: string
  passwordHash: string
  passwordVersion: number
  changedAt: string
  failedAttempts: number
  lockedUntil?: string | null
}

export interface AuthTransaction {
  id: string
  email: string
  purpose: 'login' | 'enroll-passkey' | 'step-up' | 'recovery' | 'admin-recovery'
  challenge?: string
  passwordVerified?: boolean
  requiredPolicy: 'passkey-first' | 'password-and-passkey'
  failedAttempts: number
  createdAt: string
  expiresAt: string
  consumedAt?: string
}

export interface RecoveryCode {
  id: string
  userEmail: string
  codeHash: string
  createdAt: string
  usedAt?: string
}

export interface AuthAuditEvent {
  id: string
  userEmail: string
  type: string
  createdAt: string
  detail?: string
}

// ── MFA Devices ───────────────────────────────────────────────────────────────

export interface MfaDevice {
  id: string
  userEmail: string
  name: string
  /** AES-GCM encrypted TOTP secret. */
  secret?: string
  type?: 'totp' | 'webauthn'
  credentialId?: string
  publicKey?: string
  signCount?: number
  transports?: string[]
  deviceType?: 'singleDevice' | 'multiDevice'
  backedUp?: boolean
  lastUsedAt?: string
  revokedAt?: string
  createdAt: string
}

// ── Push Notifications ───────────────────────────────────────────────────────

export interface PushSubscriptionRecord {
  id: string
  userEmail: string
  endpoint: string
  keys: {
    p256dh: string
    auth: string
  }
  createdAt: string
  updatedAt: string
  userAgent?: string
}

// ── Send ──────────────────────────────────────────────────────────────────────

export interface SendRequest {
  from?: string
  to: string[]
  cc?: string[]
  bcc?: string[]
  subject: string
  text?: string
  html?: string
  replyTo?: string[]
  messageId?: string
  inReplyTo?: string
  references?: string[]
}

export interface FollowUpReminder {
  id: string
  ownerEmail: string
  domain: string
  emailId: string
  recipient: string
  subject: string
  threadKey: string
  threadId?: string
  remindAt: number
  createdAt: number
  status: 'scheduled' | 'due' | 'completed' | 'dismissed'
  completedAt?: number
  dueAt?: number
  noticeEmailId?: string
}

// ── Inbox Rules ───────────────────────────────────────────────────────────────

export interface InboxRule {
  id: string
  ownerEmail?: string
  mailbox?: string
  domain: string
  name: string
  enabled: boolean
  order?: number
  stopProcessing?: boolean
  conditionMatch: 'all' | 'any'
  conditions: RuleCondition[]
  actions: InboxRuleAction[]
}

export type RuleCondition =
  | { field: 'from';    op: 'contains' | 'notContains' | 'equals' | 'notEquals' | 'startsWith' | 'endsWith'; value: string }
  | { field: 'to';      op: 'contains' | 'notContains' | 'equals' | 'notEquals' | 'startsWith' | 'endsWith'; value: string }
  | { field: 'subject'; op: 'contains' | 'notContains' | 'equals' | 'notEquals' | 'startsWith' | 'endsWith'; value: string }
  | { field: 'body';    op: 'contains' | 'notContains' | 'equals' | 'notEquals' | 'startsWith' | 'endsWith'; value: string }
  | { field: 'cc';      op: 'contains' | 'notContains' | 'equals' | 'notEquals' | 'startsWith' | 'endsWith'; value: string }
  | { field: 'hasAttachment'; op: 'equals' | 'notEquals'; value: 'true' | 'false' }

export type InboxRuleAction =
  | { type: 'folder';  folder: string }
  | { type: 'forward'; to: string }
  | { type: 'delete' }
  | { type: 'markRead'; value: boolean }
  | { type: 'star'; value: boolean }

/** Persisted domain row; pre-migration records may contain an array. */
export type RawDomainDoc = Omit<DomainConfig, 'routes'> & { routes?: string | RouteRule[] }

/** Persisted inbox-rule row; decoded API values use the InboxRule contract. */
export type RawRuleDoc = Omit<InboxRule, 'conditions' | 'actions'> & {
  conditions?: string | RuleCondition[]
  actions?: string | InboxRuleAction[]
}
