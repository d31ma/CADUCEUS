import { Fylo } from '../../../shared/persistence/Fylo.js'
import { McpAuditEventsRecords } from '../../../shared/mcp/McpAuditEventsRecords.js'

export class McpAuditEventsRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {import('../../../types').McpAuditEvent} event */
  async put(event) {
    await this.fylo.putData(Fylo.Collections.MCP_AUDIT_EVENTS, event)
    const records = await this.listForOwner(event.ownerEmail, Number.MAX_SAFE_INTEGER)
    for (const stale of records.slice(McpAuditEventsRecords.RETAINED_EVENTS)) await this.fylo.delDoc(Fylo.Collections.MCP_AUDIT_EVENTS, stale.docId)
  }

  /** @param {string} ownerEmail @param {number} [limit] @returns {Promise<Array<import('../../../types').McpAuditEvent & {docId:string, parameterKeys:string[]}>>} */
  async listForOwner(ownerEmail, limit = McpAuditEventsRecords.RETURNED_EVENTS) {
    const docs = await Fylo.collect(this.fylo.findDocs(Fylo.Collections.MCP_AUDIT_EVENTS, {
      $ops: [{ ownerEmail: { $eq: String(ownerEmail).toLowerCase() } }],
    }).collect())
    return Object.entries(docs)
      .map(([docId, value]) => {
        const event = /** @type {import('../../../types').McpAuditEvent} */ (value)
        return { docId, ...event, parameterKeys: McpAuditEventsRecords.parseKeys(event.parameterKeysJson) }
      })
      .sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))
      .slice(0, Math.max(0, limit))
  }

  /** @param {string} ownerEmail */
  async clearForOwner(ownerEmail) {
    const records = await this.listForOwner(ownerEmail, Number.MAX_SAFE_INTEGER)
    for (const record of records) await this.fylo.delDoc(Fylo.Collections.MCP_AUDIT_EVENTS, record.docId)
    return records.length
  }
}

