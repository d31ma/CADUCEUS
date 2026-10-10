import { Fylo } from '../shared/persistence/Fylo.js'
import { PushSubscriptionsRecords } from '../shared/notifications/PushSubscriptionsRecords.js'

/** @import { PushSubscriptionRecord } from '../types' */

@Helper
export default class PushSubscriptionsHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
   * @param {string} userEmail
   * @returns {Promise<Array<PushSubscriptionRecord & { docId: string }>>}
   */
  async listForUser(userEmail) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.PUSH_SUBSCRIPTIONS, {
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
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.PUSH_SUBSCRIPTIONS, { $ops: [{ id: { $eq: id } }] }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], /** @type {PushSubscriptionRecord} */ (entry[1])] : [null, null]
  }

  /**
   * @param {PushSubscriptionRecord} subscription
   * @returns {Promise<PushSubscriptionRecord>}
   */
  async upsert(subscription) {
    const fylo = await this.fyloHelper.database()
    const now = new Date().toISOString()
    const id = PushSubscriptionsRecords.subscriptionId(subscription.endpoint)
    const [docId, existing] = await this.findById(id)
    const record = {
      ...subscription,
      userEmail: subscription.userEmail.toLowerCase(),
      id,
      createdAt: /** @type {any} */ (existing)?.createdAt ?? now,
      updatedAt: now,
    }
    if (docId) await fylo.patchDoc(Fylo.Collections.PUSH_SUBSCRIPTIONS, { [docId]: record })
    else await fylo.putData(Fylo.Collections.PUSH_SUBSCRIPTIONS, record)
    return record
  }

  /**
   * @param {string} endpoint
   * @returns {Promise<boolean>}
   */
  async deleteByEndpoint(endpoint) {
    const fylo = await this.fyloHelper.database()
    const [docId] = await this.findById(PushSubscriptionsRecords.subscriptionId(endpoint))
    if (!docId) return false
    await fylo.delDoc(Fylo.Collections.PUSH_SUBSCRIPTIONS, docId)
    return true
  }

  /**
   * @param {string} docId
   * @returns {Promise<void>}
   */
  async deleteDoc(docId) {
    const fylo = await this.fyloHelper.database()
    await fylo.delDoc(Fylo.Collections.PUSH_SUBSCRIPTIONS, docId)
  }

}

