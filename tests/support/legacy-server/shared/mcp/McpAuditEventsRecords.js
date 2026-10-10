import { Fylo } from '../persistence/Fylo.js'

export class McpAuditEventsRecords {
  static RETAINED_EVENTS = 100

  static RETURNED_EVENTS = 50

  static parseKeys(value) {
    try {
      const parsed = JSON.parse(String(value || '[]'))
      return Array.isArray(parsed) ? parsed.filter(item => typeof item === 'string') : []
    } catch { return [] }
  }
}
