import { Collections, collect } from '../../../platform/persistence/fylo.js'

/** @typedef {{ query?: string, folder?: string, folderByDomain?: Record<string, string>, read?: boolean, starred?: boolean, hasAttachment?: boolean, limit?: number, offset?: number, since?: string, ownerEmail?: string }} EmailListFilters */
/** @typedef {import('../../../types').StoredEmail} StoredEmail */
/** @typedef {import('../../../types').EmailAttachmentSummary} EmailAttachmentSummary */
/** @typedef {import('./attachments.js').AttachmentsRepository} AttachmentsRepository */
/** @typedef {import('./raw-messages.js').RawMessagesRepository} RawMessagesRepository */

export class EmailsRepository {
  /**
   * @param {import('../../../platform/persistence/fylo.js').Fylo} fylo
   * @param {AttachmentsRepository} attachments
   * @param {RawMessagesRepository} [rawMessages]
   */
  constructor(fylo, attachments, rawMessages) {
    this.fylo = fylo
    this.attachments = attachments
    this.rawMessages = rawMessages
  }

  /**
   * Return emails whose domain appears in `allowedDomains`, newest-first.
   * @param {string[]} allowedDomains
   * @param {EmailListFilters} [filters]
   * @returns {Promise<StoredEmail[]>}
   */
  async list(allowedDomains, filters = {}) {
    const docs = await collect(this.fylo.findDocs(Collections.EMAILS, { $ops: [] }).collect())
    const needsAttachments = !!filters.query || filters.hasAttachment != null
    const attachmentsByEmail = needsAttachments
      ? await this.attachments.listSummariesByEmail(allowedDomains)
      : new Map()
    const offset = Math.max(0, filters.offset ?? 0)
    const limit = filters.limit == null ? undefined : Math.max(0, filters.limit)
    const emails = Object.values(docs)
      .map(d => normalize(/** @type {StoredEmail} */ (d)))
      .filter(e => allowedDomains.includes(e.domain))
      .filter(e => !filters.ownerEmail || canAccessEmail(e, filters.ownerEmail))
      .filter(e => matchesFolder(e, filters.folder, filters.folderByDomain))
      .filter(e => filters.read == null || e.read === filters.read)
      .filter(e => filters.starred == null || e.starred === filters.starred)
      .filter(e => matchesHasAttachment(e, filters.hasAttachment, attachmentsByEmail))
      .filter(e => matchesQuery(e, filters.query, attachmentsByEmail.get(e.id) ?? []))
      .filter(e => filters.since == null || e.receivedAt > filters.since)
      .sort((a, b) => b.receivedAt.localeCompare(a.receivedAt))
    return limit == null ? emails.slice(offset) : emails.slice(offset, offset + limit)
  }

  /**
   * Iterate one folder through FYLO's bounded immutable query pages.
   *
   * This is intentionally a repository concern: callers receive normalized,
   * mailbox-authorized records without knowing FYLO cursor syntax. A cursor
   * failure aborts the consumer, which is safer than silently restarting an
   * export and duplicating messages.
   *
   * @param {string[]} allowedDomains
   * @param {string} folder
   * @param {string} ownerEmail
   * @param {number} [pageSize]
   * @returns {AsyncGenerator<StoredEmail>}
   */
  async *iterateFolder(allowedDomains, folder, ownerEmail, pageSize = 128) {
    const domains = [...new Set(allowedDomains.map(value => String(value).toLowerCase()))]
    const selectedFolder = String(folder || 'inbox').toLowerCase()
    const boundedPageSize = Math.max(1, Math.min(512, Math.trunc(pageSize) || 128))

    for (const domain of domains) {
      let cursor
      do {
        const page = await this.fylo.findDocsPage(
          Collections.EMAILS,
          { $ops: [{ domain: { $eq: domain } }] },
          { limit: boundedPageSize, ...(cursor ? { cursor } : {}) },
        )
        for (const document of Object.values(page?.items || {})) {
          const email = normalize(/** @type {StoredEmail} */ (document))
          if ((email.folder || 'inbox') !== selectedFolder) continue
          if (!canAccessEmail(email, ownerEmail)) continue
          yield email
        }
        cursor = typeof page?.nextCursor === 'string' && page.nextCursor
          ? page.nextCursor
          : undefined
      } while (cursor)
    }
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, StoredEmail | null]>}
   */
  async findById(id) {
    const docs = await collect(this.fylo.findDocs(Collections.EMAILS, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], normalize(/** @type {StoredEmail} */ (entry[1]))] : [null, null]
  }

  /**
   * @param {StoredEmail} email
   * @returns {Promise<string>}
   */
  async put(email) {
    return await this.fylo.putData(Collections.EMAILS, email)
  }

  /**
   * @param {string} docId
   * @param {Partial<StoredEmail>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    await this.fylo.patchDoc(Collections.EMAILS, { [docId]: patch })
  }

  /**
   * Patch a set of logical message ids in one FYLO collection operation.
   * Returns the pre-patch records that were found so callers can make
   * follow-up decisions without widening their mailbox query.
   *
   * @param {string[]} ids
   * @param {Partial<StoredEmail>} patch
   * @returns {Promise<StoredEmail[]>}
   */
  async updateByIds(ids, patch) {
    const wanted = new Set(ids)
    if (wanted.size === 0) return []
    const docs = await collect(this.fylo.findDocs(Collections.EMAILS, { $ops: [] }).collect())
    const updates = {}
    const matched = []
    for (const [docId, value] of Object.entries(docs)) {
      const email = normalize(/** @type {StoredEmail} */ (value))
      if (!wanted.has(email.id)) continue
      updates[docId] = patch
      matched.push(email)
    }
    if (Object.keys(updates).length > 0) await this.fylo.patchDoc(Collections.EMAILS, updates)
    return matched
  }

  /**
   * @param {string} docId
   * @param {string} emailId
   * @returns {Promise<void>}
   */
  async delete(docId, emailId) {
    if (emailId) await this.attachments.deleteForEmail(emailId)
    if (emailId) await this.rawMessages?.deleteForEmail(emailId)
    await this.fylo.delDoc(Collections.EMAILS, docId)
  }

  static normalize = normalize
}

/**
 * Enforce exact mailbox ownership below every caller-controlled query.
 *
 * `ownerEmail` became part of the stored record in schema v3. When it is
 * present it is authoritative: being another visible participant never grants
 * access to that mailbox's copy. Historical records have a deliberately
 * narrow compatibility path. Sent mail belongs to its sender, drafts belong
 * to their draft owner, and every other folder belongs to an envelope or
 * visible recipient. Ambiguous legacy rows fail closed.
 *
 * @param {StoredEmail} email
 * @param {string} ownerEmail
 */
export function canAccessEmail(email, ownerEmail) {
  const owner = normalizeAddress(ownerEmail)
  if (!owner) return false

  if (email.ownerEmail != null && String(email.ownerEmail).trim()) {
    const explicitOwner = normalizeAddress(email.ownerEmail)
    return explicitOwner ? explicitOwner === owner : false
  }

  const folder = String(email.folder || 'inbox').toLowerCase()
  if (folder === 'drafts') {
    return normalizeAddress(email.draftOwner) === owner || normalizeAddress(email.sender) === owner
  }
  if (folder === 'sent') return normalizeAddress(email.sender) === owner

  return recipientAddresses(email).includes(owner)
}

/** @param {StoredEmail} email @returns {string[]} */
function recipientAddresses(email) {
  const values = [
    email.originalRecipient,
    email.recipient,
    ...(Array.isArray(email.to) ? email.to : []),
    ...(Array.isArray(email.cc) ? email.cc : []),
    ...(Array.isArray(email.bcc) ? email.bcc : []),
  ]
  const addresses = values.flatMap(value => String(value ?? '').match(ADDRESS_PATTERN) ?? [])
  return [...new Set(addresses.map(address => address.toLowerCase()))]
}

/** @param {unknown} value @returns {string} */
function normalizeAddress(value) {
  const addresses = String(value ?? '').match(ADDRESS_PATTERN) ?? []
  return addresses.length === 1 ? addresses[0].toLowerCase() : ''
}

const ADDRESS_PATTERN = /[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+/gi

/** @param {StoredEmail} email @returns {StoredEmail} */
function normalize(email) {
  const normalized = { ...email, folder: email.folder || 'inbox', read: email.read ?? false, starred: email.starred ?? false }
  normalized.labels = Array.isArray(email.labels) ? [...new Set(email.labels.map(value => String(value).toLowerCase()))] : []
  normalized.trackedLinks = parseJsonArray(normalized.trackedLinksJson)
  normalized.opens = parseJsonArray(normalized.opensJson)
  normalized.clicks = parseJsonArray(normalized.clicksJson || normalized.clicks)
  delete normalized.trackedLinksJson
  delete normalized.opensJson
  delete normalized.clicksJson
  if (normalized.spamScore == null) delete normalized.spamScore
  if (normalized.spamReasons == null) delete normalized.spamReasons
  return normalized
}

/** @param {unknown} value */
function parseJsonArray(value) {
  if (Array.isArray(value)) return value
  if (typeof value !== 'string' || !value) return []
  try {
    const parsed = JSON.parse(value)
    return Array.isArray(parsed) ? parsed : []
  } catch { return [] }
}

/** @param {StoredEmail} email @param {string | undefined} folder @param {Record<string, string> | undefined} folderByDomain */
function matchesFolder(email, folder, folderByDomain) {
  const selected = folderByDomain?.[email.domain] || folder
  return !selected || selected === 'all' || (email.folder || 'inbox') === selected
}

/** @param {StoredEmail} email @param {boolean | undefined} hasAttachment @param {Map<string, EmailAttachmentSummary[]>} attachmentsByEmail */
function matchesHasAttachment(email, hasAttachment, attachmentsByEmail) {
  if (hasAttachment == null) return true
  const has = (attachmentsByEmail.get(email.id)?.length ?? 0) > 0
  return has === hasAttachment
}

/** @param {StoredEmail} email @param {string | undefined} query @param {EmailAttachmentSummary[]} attachments */
function matchesQuery(email, query, attachments) {
  const terms = (query ?? '').trim().toLowerCase().split(/\s+/).filter(Boolean)
  if (terms.length === 0) return true
  return terms.every(term => matchesTerm(email, term, attachments))
}

/** @param {StoredEmail} email @param {string} term @param {EmailAttachmentSummary[]} attachments */
function matchesTerm(email, term, attachments) {
  if (term === 'has:attachment') return attachments.length > 0
  if (term === 'is:read') return email.read
  if (term === 'is:unread') return !email.read
  if (term === 'is:starred') return email.starred
  const colon = term.indexOf(':')
  if (colon > 0) {
    const field = term.slice(0, colon)
    const value = term.slice(colon + 1)
    if (value) {
      if (field === 'from') return includes(email.sender, value)
      if (field === 'to') return includes(email.recipient, value)
      if (field === 'subject') return includes(email.subject, value)
      if (field === 'body') return includes(email.body, value)
      if (field === 'filename' || field === 'attachment') return attachments.some(a => includes(a.filename, value))
    }
  }
  const haystacks = [email.sender, email.recipient, email.subject, email.body, ...attachments.map(a => a.filename)]
  return haystacks.some(field => includes(field, term))
}

/** @param {string | undefined} value @param {string} term */
function includes(value, term) {
  return (value ?? '').toLowerCase().includes(term)
}
