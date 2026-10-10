import { Fylo } from '../../../shared/persistence/Fylo.js'

export class McpAccessTokensRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {import('../../../types').McpAccessTokenRecord} record */
  async put(record) { return await this.fylo.putData(Fylo.Collections.MCP_ACCESS_TOKENS, record) }

  /** @param {string} ownerEmail @returns {Promise<Array<import('../../../types').McpAccessTokenRecord & {docId:string}>>} */
  async listForOwner(ownerEmail) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.MCP_ACCESS_TOKENS, {
      $ops: [{ ownerEmail: { $eq: String(ownerEmail).toLowerCase() } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({
      docId,
      .../** @type {import('../../../types').McpAccessTokenRecord} */ (value),
    }))
  }

  /** @param {string} tokenHash @returns {Promise<Array<import('../../../types').McpAccessTokenRecord & {docId:string}>>} */
  async findByHash(tokenHash) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.MCP_ACCESS_TOKENS, {
      $ops: [{ tokenHash: { $eq: tokenHash } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({
      docId,
      .../** @type {import('../../../types').McpAccessTokenRecord} */ (value),
    }))
  }

  /** @param {string} docId @param {Partial<import('../../../types').McpAccessTokenRecord>} patch */
  async update(docId, patch) {
    await this.fylo.patchDoc(Fylo.Collections.MCP_ACCESS_TOKENS, { [docId]: patch })
  }

  /** @param {string} docId */
  async delete(docId) { await this.fylo.delDoc(Fylo.Collections.MCP_ACCESS_TOKENS, docId) }
}
