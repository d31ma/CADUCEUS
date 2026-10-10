import { Fylo } from '../shared/persistence/Fylo.js'

@Helper
export default class McpAccessTokensHelper {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /** @param {import('../types').McpAccessTokenRecord} record */
  async put(record) {
    const fylo = await this.fyloHelper.database(); return await fylo.putData(Fylo.Collections.MCP_ACCESS_TOKENS, record) }

  /** @param {string} ownerEmail @returns {Promise<Array<import('../types').McpAccessTokenRecord & {docId:string}>>} */
  async listForOwner(ownerEmail) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.MCP_ACCESS_TOKENS, {
      $ops: [{ ownerEmail: { $eq: String(ownerEmail).toLowerCase() } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({
      docId,
      .../** @type {import('../types').McpAccessTokenRecord} */ (value),
    }))
  }

  /** @param {string} tokenHash @returns {Promise<Array<import('../types').McpAccessTokenRecord & {docId:string}>>} */
  async findByHash(tokenHash) {
    const fylo = await this.fyloHelper.database()
    const docs = await Fylo.collect(fylo.findDocs(Fylo.Collections.MCP_ACCESS_TOKENS, {
      $ops: [{ tokenHash: { $eq: tokenHash } }],
    }).collect())
    return Object.entries(docs).map(([docId, value]) => ({
      docId,
      .../** @type {import('../types').McpAccessTokenRecord} */ (value),
    }))
  }

  /** @param {string} docId @param {Partial<import('../types').McpAccessTokenRecord>} patch */
  async update(docId, patch) {
    const fylo = await this.fyloHelper.database()
    await fylo.patchDoc(Fylo.Collections.MCP_ACCESS_TOKENS, { [docId]: patch })
  }

  /** @param {string} docId */
  async delete(docId) {
    const fylo = await this.fyloHelper.database(); await fylo.delDoc(Fylo.Collections.MCP_ACCESS_TOKENS, docId) }
}
