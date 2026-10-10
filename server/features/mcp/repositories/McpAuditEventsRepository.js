import { Collections, collect } from '../../../shared/persistence/Fylo.js'

const RETAINED_EVENTS = 100
const RETURNED_EVENTS = 50

export class McpAuditEventsRepository {
  /** @param {import('../../../shared/persistence/Fylo.js').Fylo} fylo */
  constructor(fylo) { this.fylo = fylo }

  /** @param {import('../../../types').McpAuditEvent} event */
  async put(event) {
    await this.fylo.putData(Collections.MCP_AUDIT_EVENTS, event)
    const records = await this.listForOwner(event.ownerEmail, Number.MAX_SAFE_INTEGER)
    for (const stale of records.slice(RETAINED_EVENTS)) await this.fylo.delDoc(Collections.MCP_AUDIT_EVENTS, stale.docId)
  }

  /** @param {string} ownerEmail @param {number} [limit] @returns {Promise<Array<import('../../../types').McpAuditEvent & {docId:string, parameterKeys:string[]}>>} */
  async listForOwner(ownerEmail, limit = RETURNED_EVENTS) {
    const docs = await collect(this.fylo.findDocs(Collections.MCP_AUDIT_EVENTS, {
      $ops: [{ ownerEmail: { $eq: String(ownerEmail).toLowerCase() } }],
    }).collect())
    return Object.entries(docs)
      .map(([docId, value]) => {
        const event = /** @type {import('../../../types').McpAuditEvent} */ (value)
        return { docId, ...event, parameterKeys: parseKeys(event.parameterKeysJson) }
      })
      .sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt)))
      .slice(0, Math.max(0, limit))
  }

  /** @param {string} ownerEmail */
  async clearForOwner(ownerEmail) {
    const records = await this.listForOwner(ownerEmail, Number.MAX_SAFE_INTEGER)
    for (const record of records) await this.fylo.delDoc(Collections.MCP_AUDIT_EVENTS, record.docId)
    return records.length
  }
}

function parseKeys(value) {
  try {
    const parsed = JSON.parse(String(value || '[]'))
    return Array.isArray(parsed) ? parsed.filter(item => typeof item === 'string') : []
  } catch { return [] }
}
