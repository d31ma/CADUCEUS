import { Collections, collect } from '../../../platform/persistence/fylo.js'

/** FYLO persistence for mailbox-owned conversation capabilities. */
export class ThreadSharesRepository {
  /** @param {import('../../../platform/persistence/fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {ThreadShareRecord} share */
  async put(share) {
    return await this.fylo.putData(Collections.THREAD_SHARES, serialize(share))
  }

  /** @param {string} ownerEmail @returns {Promise<Array<ThreadShareRecord & { docId: string }>>} */
  async listForOwner(ownerEmail) {
    const docs = await collect(this.fylo.findDocs(Collections.THREAD_SHARES, {
      $ops: [{ ownerEmail: { $eq: ownerEmail.toLowerCase() } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({ docId, ...deserialize(value) }))
  }

  /** @param {string} id @returns {Promise<[string | null, ThreadShareRecord | null]>} */
  async findById(id) {
    const docs = await collect(this.fylo.findDocs(Collections.THREAD_SHARES, {
      $ops: [{ id: { $eq: id } }],
    }).collect())
    const entry = Object.entries(docs)[0]
    return entry ? [entry[0], deserialize(entry[1])] : [null, null]
  }

  /** @param {string} tokenHash @returns {Promise<Array<ThreadShareRecord & { docId: string }>>} */
  async findByTokenHash(tokenHash) {
    const docs = await collect(this.fylo.findDocs(Collections.THREAD_SHARES, {
      $ops: [{ tokenHash: { $eq: tokenHash } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({ docId, ...deserialize(value) }))
  }

  /** @param {string} docId @param {Partial<ThreadShareRecord>} patch */
  async update(docId, patch) {
    const stored = /** @type {Record<string, any>} */ ({ ...patch })
    if (patch.emailIds) {
      stored.emailIdsJson = JSON.stringify(patch.emailIds)
      delete stored.emailIds
    }
    await this.fylo.patchDoc(Collections.THREAD_SHARES, { [docId]: stored })
  }
}

/** @param {ThreadShareRecord} share */
function serialize(share) {
  const { emailIds, ...stored } = share
  return { ...stored, emailIdsJson: JSON.stringify(emailIds) }
}

/** @param {any} value @returns {ThreadShareRecord} */
function deserialize(value) {
  let emailIds = []
  try {
    const parsed = JSON.parse(value.emailIdsJson || '[]')
    if (Array.isArray(parsed)) emailIds = parsed.filter(item => typeof item === 'string')
  } catch {}
  const result = { ...value, emailIds }
  delete result.emailIdsJson
  return result
}
