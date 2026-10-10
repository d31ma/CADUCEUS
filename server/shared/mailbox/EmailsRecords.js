import { Fylo } from '../persistence/Fylo.js'

export class EmailsRecords {
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
  static canAccessEmail(email, ownerEmail) {
    const owner = EmailsRecords.normalizeAddress(ownerEmail)
    if (!owner) return false

    if (email.ownerEmail != null && String(email.ownerEmail).trim()) {
      const explicitOwner = EmailsRecords.normalizeAddress(email.ownerEmail)
      return explicitOwner ? explicitOwner === owner : false
    }

    const folder = String(email.folder || 'inbox').toLowerCase()
    if (folder === 'drafts') {
      return EmailsRecords.normalizeAddress(email.draftOwner) === owner || EmailsRecords.normalizeAddress(email.sender) === owner
    }
    if (folder === 'sent') return EmailsRecords.normalizeAddress(email.sender) === owner

    return EmailsRecords.recipientAddresses(email).includes(owner)
  }

  /** @param {StoredEmail} email @returns {string[]} */
  static recipientAddresses(email) {
    const values = [
      email.originalRecipient,
      email.recipient,
      ...(Array.isArray(email.to) ? email.to : []),
      ...(Array.isArray(email.cc) ? email.cc : []),
      ...(Array.isArray(email.bcc) ? email.bcc : []),
    ]
    const addresses = values.flatMap(value => String(value ?? '').match(EmailsRecords.ADDRESS_PATTERN) ?? [])
    return [...new Set(addresses.map(address => address.toLowerCase()))]
  }

  /** @param {unknown} value @returns {string} */
  static normalizeAddress(value) {
    const addresses = String(value ?? '').match(EmailsRecords.ADDRESS_PATTERN) ?? []
    return addresses.length === 1 ? addresses[0].toLowerCase() : ''
  }

  static ADDRESS_PATTERN = /[A-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?(?:\.[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?)+/gi

  /** @param {StoredEmail} email @returns {StoredEmail} */
  static normalize(email) {
    const normalized = { ...email, folder: email.folder || 'inbox', read: email.read ?? false, starred: email.starred ?? false }
    normalized.labels = Array.isArray(email.labels) ? [...new Set(email.labels.map(value => String(value).toLowerCase()))] : []
    normalized.trackedLinks = EmailsRecords.parseJsonArray(normalized.trackedLinksJson)
    normalized.opens = EmailsRecords.parseJsonArray(normalized.opensJson)
    normalized.clicks = EmailsRecords.parseJsonArray(normalized.clicksJson || normalized.clicks)
    delete normalized.trackedLinksJson
    delete normalized.opensJson
    delete normalized.clicksJson
    if (normalized.spamScore == null) delete normalized.spamScore
    if (normalized.spamReasons == null) delete normalized.spamReasons
    return normalized
  }

  /** @param {unknown} value */
  static parseJsonArray(value) {
    if (Array.isArray(value)) return value
    if (typeof value !== 'string' || !value) return []
    try {
      const parsed = JSON.parse(value)
      return Array.isArray(parsed) ? parsed : []
    } catch { return [] }
  }

  /** @param {StoredEmail} email @param {string | undefined} folder @param {Record<string, string> | undefined} folderByDomain */
  static matchesFolder(email, folder, folderByDomain) {
    const selected = folderByDomain?.[email.domain] || folder
    return !selected || selected === 'all' || (email.folder || 'inbox') === selected
  }

  /** @param {StoredEmail} email @param {boolean | undefined} hasAttachment @param {Map<string, EmailAttachmentSummary[]>} attachmentsByEmail */
  static matchesHasAttachment(email, hasAttachment, attachmentsByEmail) {
    if (hasAttachment == null) return true
    const has = (attachmentsByEmail.get(email.id)?.length ?? 0) > 0
    return has === hasAttachment
  }

  /** @param {StoredEmail} email @param {string | undefined} query @param {EmailAttachmentSummary[]} attachments */
  static matchesQuery(email, query, attachments) {
    const terms = (query ?? '').trim().toLowerCase().split(/\s+/).filter(Boolean)
    if (terms.length === 0) return true
    return terms.every(term => EmailsRecords.matchesTerm(email, term, attachments))
  }

  /** @param {StoredEmail} email @param {string} term @param {EmailAttachmentSummary[]} attachments */
  static matchesTerm(email, term, attachments) {
    if (term === 'has:attachment') return attachments.length > 0
    if (term === 'is:read') return email.read
    if (term === 'is:unread') return !email.read
    if (term === 'is:starred') return email.starred
    const colon = term.indexOf(':')
    if (colon > 0) {
      const field = term.slice(0, colon)
      const value = term.slice(colon + 1)
      if (value) {
        if (field === 'from') return EmailsRecords.includes(email.sender, value)
        if (field === 'to') return EmailsRecords.includes(email.recipient, value)
        if (field === 'subject') return EmailsRecords.includes(email.subject, value)
        if (field === 'body') return EmailsRecords.includes(email.body, value)
        if (field === 'filename' || field === 'attachment') return attachments.some(a => EmailsRecords.includes(a.filename, value))
      }
    }
    const haystacks = [email.sender, email.recipient, email.subject, email.body, ...attachments.map(a => a.filename)]
    return haystacks.some(field => EmailsRecords.includes(field, term))
  }

  /** @param {string | undefined} value @param {string} term */
  static includes(value, term) {
    return (value ?? '').toLowerCase().includes(term)
  }
}
