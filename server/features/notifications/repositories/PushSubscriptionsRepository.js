import { createHash } from 'node:crypto'
import { Collections, collect } from '../../../platform/persistence/Fylo.js'
/** @import { PushSubscriptionRecord } from '../../../types' */

export class PushSubscriptionsRepository {
  /** @param {import('../../../platform/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) {
    this.fylo = fylo
  }

  /**
   * @param {string} userEmail
   * @returns {Promise<Array<PushSubscriptionRecord & { docId: string }>>}
   */
  async listForUser(userEmail) {
    const docs = await collect(this.fylo.findDocs(Collections.PUSH_SUBSCRIPTIONS, {
      $ops: [{ userEmail: { $eq: userEmail.toLowerCase() } }],
    }).collect())
    return /** @type {Array<PushSubscriptionRecord & { docId: string }>} */ (
      Object.entries(docs).map(([docId, sub]) => ({ docId, ...sub }))
    )
  }

  /**
   * @param {string} address
   * @returns {Promise<Array<PushSubscriptionRecord & { docId: string }>>}
   */
  async listForAddress(address) {
    return this.listForUser(address.toLowerCase())
  }

  /**
   * @param {string} id
   * @returns {Promise<[string | null, PushSubscriptionRecord | null]>}
   */
  async findById(id) {
    const docs = await collect(this.fylo.findDocs(Collections.PUSH_SUBSCRIPTIONS, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {PushSubscriptionRecord} */ (entry[1])] : [null, null]
  }

  /**
   * @param {PushSubscriptionRecord} subscription
   * @returns {Promise<PushSubscriptionRecord>}
   */
  async upsert(subscription) {
    const now = new Date().toISOString()
    const id = subscriptionId(subscription.endpoint)
    const [docId, existing] = await this.findById(id)
    const record = {
      ...subscription,
      userEmail: subscription.userEmail.toLowerCase(),
      id,
      createdAt: /** @type {any} */ (existing)?.createdAt ?? now,
      updatedAt: now,
    }
    if (docId) await this.fylo.patchDoc(Collections.PUSH_SUBSCRIPTIONS, { [docId]: record })
    else await this.fylo.putData(Collections.PUSH_SUBSCRIPTIONS, record)
    return record
  }

  /**
   * @param {string} endpoint
   * @returns {Promise<boolean>}
   */
  async deleteByEndpoint(endpoint) {
    const [docId] = await this.findById(subscriptionId(endpoint))
    if (!docId) return false
    await this.fylo.delDoc(Collections.PUSH_SUBSCRIPTIONS, docId)
    return true
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async deleteDoc(docId) {
    await this.fylo.delDoc(Collections.PUSH_SUBSCRIPTIONS, docId)
  }

  /** @param {string} endpoint @returns {string} */
  static subscriptionId(endpoint) {
    return subscriptionId(endpoint)
  }
}

/** @param {string} endpoint @returns {string} */
function subscriptionId(endpoint) {
  return createHash('sha256').update(endpoint).digest('hex')
}
