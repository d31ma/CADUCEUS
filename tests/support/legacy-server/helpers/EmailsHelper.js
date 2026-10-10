import { Fylo } from '../shared/persistence/Fylo.js'
import { EmailsRecords } from '../shared/mailbox/EmailsRecords.js'

/** @import { StoredEmail } from '../types' */
/** @import { EmailAttachmentSummary } from '../types' */
/** @import { AttachmentsHelper } from './AttachmentsHelper.js' */
/** @import { RawMessagesHelper } from './RawMessagesHelper.js' */

@Helper
export default class EmailsHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /** @type {AttachmentsHelper} */
  @Needs
  attachmentsHelper

  /** @type {RawMessagesHelper} */
  @Needs
  rawMessagesHelper

  /**
   * Return emails whose domain appears in `allowedDomains`, newest-first.
   * @param {string[]} allowedDomains
   * @param {EmailListFilters} [filters]
   * @returns {Promise<StoredEmail[]>}
   */
  async list(allowedDomains, filters = {}) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.EMAILS, { $ops: [] }).collect())
    const needsAttachments = !!filters.query || filters.hasAttachment != null
    const attachmentsByEmail = needsAttachments
      ? await this.attachmentsHelper.listSummariesByEmail(allowedDomains)
      : new Map()
    const offset = Math.max(0, filters.offset ?? 0)
    const limit = filters.limit == null ? undefined : Math.max(0, filters.limit)
    const emails = Object.values(docs)
      .map(d => EmailsRecords.normalize(/** @type {StoredEmail} */ (d)))
      .filter(e => allowedDomains.includes(e.domain))
      .filter(e => !filters.ownerEmail || EmailsRecords.canAccessEmail(e, filters.ownerEmail))
      .filter(e => EmailsRecords.matchesFolder(e, filters.folder, filters.folderByDomain))
      .filter(e => filters.read == null || e.read === filters.read)
      .filter(e => filters.starred == null || e.starred === filters.starred)
      .filter(e => EmailsRecords.matchesHasAttachment(e, filters.hasAttachment, attachmentsByEmail))
      .filter(e => EmailsRecords.matchesQuery(e, filters.query, attachmentsByEmail.get(e.id) ?? []))
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
    const fylo = await this.fyloHelper.database()
    const domains = [...new Set(allowedDomains.map(value => String(value).toLowerCase()))]
    const selectedFolder = String(folder || 'inbox').toLowerCase()
    const boundedPageSize = Math.max(1, Math.min(512, Math.trunc(pageSize) || 128))

    for (const domain of domains) {
      let cursor
      do {
        const page = await fylo.findDocsPage(
          Fylo.Collections.EMAILS,
          { $ops: [{ domain: { $eq: domain } }] },
          { limit: boundedPageSize, ...(cursor ? { cursor } : {}) },
        )
        for (const document of Object.values(page?.items || {})) {
          const email = EmailsRecords.normalize(/** @type {StoredEmail} */ (document))
          if ((email.folder || 'inbox') !== selectedFolder) continue
          if (!EmailsRecords.canAccessEmail(email, ownerEmail)) continue
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
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.EMAILS, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], EmailsRecords.normalize(/** @type {StoredEmail} */ (entry[1]))] : [null, null]
  }

  /**
   * @param {StoredEmail} email
   * @returns {Promise<string>}
   */
  async put(email) {
    const fylo = await this.fyloHelper.database()
    return await fylo.putData(Fylo.Collections.EMAILS, email)
  }

  /**
   * @param {string} docId
   * @param {Partial<StoredEmail>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.EMAILS, { [docId]: patch })
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
    const fylo = await this.fyloHelper.database()
    const wanted = new Set(ids)
    if (wanted.size === 0) return []
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.EMAILS, { $ops: [] }).collect())
    const updates = {}
    const matched = []
    for (const [docId, value] of Object.entries(docs)) {
      const email = EmailsRecords.normalize(/** @type {StoredEmail} */ (value))
      if (!wanted.has(email.id)) continue
      updates[docId] = patch
      matched.push(email)
    }
    if (Object.keys(updates).length > 0) await fylo.patchDoc(Fylo.Collections.EMAILS, updates)
    return matched
  }

  /**
   * @param {string} docId
   * @param {string} emailId
   * @returns {Promise<void>}
   */
  async delete(docId, emailId) {
    const fylo = await this.fyloHelper.database()
    if (emailId) await this.attachmentsHelper.deleteForEmail(emailId)
    if (emailId) await this.rawMessagesHelper?.deleteForEmail(emailId)
    await fylo.delDoc(Fylo.Collections.EMAILS, docId)
  }

  static normalize = EmailsRecords.normalize
}

