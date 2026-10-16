/** @import { RawRuleDoc } from '../types' */
import { Fylo } from '../shared/persistence/Fylo.js'
import { RulesRecords } from '../shared/mailbox/RulesRecords.js'

/** @import { InboxRule } from '../types' */

@Helper
export default class RulesHelper {
  /** @type {FyloHelper | undefined} */
  @Needs
  fyloHelper

  /**
   * @param {string[]} allowedDomains
   * @returns {Promise<InboxRule[]>}
   */
  async list(allowedDomains) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.INBOX_RULES, { $ops: [] }).collect())
    return Object.values(docs)
      .filter(r => allowedDomains.includes(r.domain))
      .map(r => RulesRecords.deserialize(/** @type {RawRuleDoc} */ (r)))
      .sort(RulesRecords.compareRules)
  }

  /** @param {string} ownerEmail @returns {Promise<InboxRule[]>} */
  async listForOwner(ownerEmail) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const owner = ownerEmail.trim().toLowerCase()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.INBOX_RULES, {
      $ops: [{ ownerEmail: { $eq: owner } }],
    }).collect())
    return Object.values(docs)
      .map(r => RulesRecords.deserialize(/** @type {RawRuleDoc} */ (r)))
      .sort(RulesRecords.compareRules)
  }

  /**
   * @param {string} domain
   * @returns {Promise<InboxRule[]>}
   */
  async listEnabledForDomain(domain) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.INBOX_RULES, { $ops: [{ domain: { $eq: domain } }] }).collect())
    return Object.values(docs)
      .filter(r => r.enabled)
      .map(r => RulesRecords.deserialize(/** @type {RawRuleDoc} */ (r)))
      .sort(RulesRecords.compareRules)
  }

  /**
   * Exact-owner rules apply to the mailbox they name. Historical v1 rows had
   * no owner, so they retain their former domain-wide behavior until an
   * operator deliberately migrates or replaces them.
   * @param {string} ownerEmail @param {string} mailbox @param {string} domain
   */
  async listEnabledForMailbox(ownerEmail, mailbox, domain) {
    const owner = ownerEmail.trim().toLowerCase()
    const address = mailbox.trim().toLowerCase()
    return (await this.listEnabledForDomain(domain))
      .filter(rule => rule.ownerEmail
        ? rule.ownerEmail === owner && (!rule.mailbox || rule.mailbox === address)
        : true)
      .sort(RulesRecords.compareRules)
  }

  /**
   * @param {string} ruleId
   * @returns {Promise<[string | null, InboxRule | null]>}
   */
  async findById(ruleId) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.INBOX_RULES, { $ops: [{ id: { $eq: ruleId } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], RulesRecords.deserialize(/** @type {RawRuleDoc} */ (entry[1]))] : [null, null]
  }

  /**
   * @param {InboxRule} rule
   * @returns {Promise<string>}
   */
  async put(rule) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    return await fylo.putData(Fylo.Collections.INBOX_RULES, RulesRecords.serialize(rule))
  }

  /**
   * @param {string} docId
   * @param {Partial<InboxRule>} patch
   * @returns {Promise<void>}
   */
  async update(docId, patch) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    /** @type {Record<string, unknown>} */
    const serialized = {}
    if (patch.name !== undefined) serialized.name = patch.name
    if (patch.enabled !== undefined) serialized.enabled = patch.enabled
    if (patch.ownerEmail !== undefined) serialized.ownerEmail = patch.ownerEmail
    if (patch.mailbox !== undefined) serialized.mailbox = patch.mailbox
    if (patch.order !== undefined) serialized.order = patch.order
    if (patch.stopProcessing !== undefined) serialized.stopProcessing = patch.stopProcessing
    if (patch.conditionMatch !== undefined) serialized.conditionMatch = patch.conditionMatch
    if (patch.conditions !== undefined) serialized.conditions = JSON.stringify(patch.conditions)
    if (patch.actions !== undefined) serialized.actions = JSON.stringify(patch.actions)
    await fylo.patchDoc(Fylo.Collections.INBOX_RULES, { [docId]: serialized })
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async delete(docId) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    await fylo.delDoc(Fylo.Collections.INBOX_RULES, docId)
  }

  /** @param {string} ownerEmail */
  async nextOrder(ownerEmail) {
    const rules = await this.listForOwner(ownerEmail)
    return rules.reduce((highest, rule) => Math.max(highest, Number(rule.order) || 0), -1) + 1
  }

  /** Replace the exact owner's complete order in one FYLO patch operation. */
  /** @param {string} ownerEmail @param {string[]} ids */
  async reorder(ownerEmail, ids) {
    const fylo = await (/** @type {FyloHelper} */ (this.fyloHelper)).database()
    const rules = await this.listForOwner(ownerEmail)
    const byId = new Map(rules.map(rule => [rule.id, rule]))
    if (new Set(ids).size !== ids.length || ids.length !== rules.length || ids.some(id => !byId.has(id))) return false
    const docs = await Promise.all(ids.map(id => this.findById(id)))
    /** @type {Record<string, { order: number }>} */
    const patch = {}
    docs.forEach(([docId], order) => { if (docId) patch[docId] = { order } })
    await fylo.patchDoc(Fylo.Collections.INBOX_RULES, patch)
    return true
  }
}

